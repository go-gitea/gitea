// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"fmt"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ErrInvalidCommitCommentLine is returned when the comment line is zero.
// Diff line numbers are signed (negative = old side, positive = new side).
var ErrInvalidCommitCommentLine = errors.New("commit comment line must be non-zero")

// CommitComment is the junction that anchors a Comment (Type=CommentTypeCommitComment)
// onto a repository commit diff coordinate. Location lives here so the main
// comment table does not need a (repo_id, commit_sha) index.
type CommitComment struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX(s) NOT NULL"`
	CommitSHA string `xorm:"VARCHAR(64) INDEX(s) NOT NULL"`
	TreePath  string `xorm:"VARCHAR(4000) NOT NULL"`
	Line      int64  `xorm:"NOT NULL"`
	CommentID int64  `xorm:"UNIQUE INDEX NOT NULL"`

	Comment *Comment `xorm:"-"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

// CreateCommitCommentOptions holds the data needed to create an inline commit comment.
type CreateCommitCommentOptions struct {
	Repo        *repo_model.Repository
	Doer        *user_model.User
	CommitSHA   string
	TreePath    string
	Line        int64 // signed: negative = previous, positive = proposed
	Content     string
	Patch       string
	Attachments []string // attachment UUIDs to bind to the Comment
}

// CreateCommitComment inserts a Comment (type CommitComment) and a junction row
// in one transaction, then binds any attachments to the comment so orphan
// cleanup (DeleteOrphanedAttachments / checkStorage) cannot delete them.
func CreateCommitComment(ctx context.Context, opts *CreateCommitCommentOptions) (*Comment, error) {
	if opts.Line == 0 {
		return nil, ErrInvalidCommitCommentLine
	}
	if opts.Repo == nil || opts.Doer == nil {
		return nil, errors.New("repo and doer are required")
	}
	if opts.CommitSHA == "" || opts.TreePath == "" || opts.Content == "" {
		return nil, errors.New("commit sha, path and content are required")
	}

	var comment *Comment
	err := db.WithTx(ctx, func(ctx context.Context) error {
		comment = &Comment{
			Type:      CommentTypeCommitComment,
			PosterID:  opts.Doer.ID,
			Poster:    opts.Doer,
			IssueID:   0,
			CommitSHA: opts.CommitSHA,
			TreePath:  opts.TreePath,
			Line:      opts.Line,
			Content:   opts.Content,
			Patch:     opts.Patch,
		}
		if err := db.Insert(ctx, comment); err != nil {
			return fmt.Errorf("insert comment: %w", err)
		}

		junction := &CommitComment{
			RepoID:    opts.Repo.ID,
			CommitSHA: opts.CommitSHA,
			TreePath:  opts.TreePath,
			Line:      opts.Line,
			CommentID: comment.ID,
			Comment:   comment,
		}
		if err := db.Insert(ctx, junction); err != nil {
			return fmt.Errorf("insert commit_comment junction: %w", err)
		}

		return UpdateCommitCommentAttachments(ctx, opts.Repo.ID, comment, opts.Attachments)
	})
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// UpdateCommitCommentAttachments binds uploaded attachment UUIDs to a commit
// comment. Binding sets comment_id (and repo_id), which is enough for:
//   - GetUnlinkedAttachmentsByUserID (requires comment_id = 0 to count as unlinked)
//   - checkStorage Attachments orphan check (ExistAttachmentsByUUID)
//   - DeleteOrphanedAttachments (only removes rows whose issue_id/release_id
//     point at missing parents — comment_id-only rows are left alone)
func UpdateCommitCommentAttachments(ctx context.Context, repoID int64, c *Comment, uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}
	if c == nil || c.ID == 0 {
		return errors.New("comment must be persisted before binding attachments")
	}
	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, uuids)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", uuids, err)
	}
	for i := range attachments {
		a := attachments[i]
		if a.RepoID != 0 && a.RepoID != repoID {
			return util.NewPermissionDeniedErrorf("attachment belongs to a different repository")
		}
		if a.IssueID != 0 {
			return util.NewPermissionDeniedErrorf("attachment is already linked to an issue")
		}
		if a.ReleaseID != 0 {
			return util.NewPermissionDeniedErrorf("attachment is already linked to a release")
		}
		if a.CommentID != 0 && a.CommentID != c.ID {
			return util.NewPermissionDeniedErrorf("attachment is already linked to another comment")
		}
		a.RepoID = repoID
		a.CommentID = c.ID
		if err := repo_model.UpdateAttachment(ctx, a); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", a.ID, err)
		}
	}
	c.Attachments = attachments
	return nil
}

// DeleteCommitComment deletes a commit comment and its junction row, scoped to repo.
func DeleteCommitComment(ctx context.Context, repoID, commentID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		comment, err := GetCommitCommentByID(ctx, repoID, commentID)
		if err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).
			Where("repo_id = ? AND comment_id = ?", repoID, commentID).
			Delete(&CommitComment{}); err != nil {
			return err
		}
		// Drop bound attachments (storage + rows) then the Comment row.
		if _, err := repo_model.DeleteAttachmentsByComment(ctx, comment.ID, true); err != nil {
			return err
		}
		_, err = db.GetEngine(ctx).ID(comment.ID).Delete(&Comment{})
		return err
	})
}

// GetCommitCommentByID returns the Comment for a commit comment, scoped to repo.
func GetCommitCommentByID(ctx context.Context, repoID, commentID int64) (*Comment, error) {
	var junction CommitComment
	has, err := db.GetEngine(ctx).
		Where("repo_id = ? AND comment_id = ?", repoID, commentID).
		Get(&junction)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{Resource: "CommitComment", ID: commentID}
	}
	comment, err := GetCommentByID(ctx, commentID)
	if err != nil {
		return nil, err
	}
	if comment.Type != CommentTypeCommitComment {
		return nil, db.ErrNotExist{Resource: "CommitComment", ID: commentID}
	}
	return comment, nil
}

func findCommitComments(ctx context.Context, cond builder.Cond) (CommentList, error) {
	var junctions []*CommitComment
	if err := db.GetEngine(ctx).
		Where(cond).
		OrderBy("id ASC").
		Find(&junctions); err != nil {
		return nil, err
	}
	if len(junctions) == 0 {
		return nil, nil
	}

	commentIDs := make([]int64, len(junctions))
	for i, j := range junctions {
		commentIDs[i] = j.CommentID
	}

	comments := make(CommentList, 0, len(commentIDs))
	if err := db.GetEngine(ctx).
		In("id", commentIDs).
		Asc("created_unix").
		Asc("id").
		Find(&comments); err != nil {
		return nil, err
	}

	// Preserve junction order (which follows creation) while filtering missing rows.
	byID := make(map[int64]*Comment, len(comments))
	for _, c := range comments {
		byID[c.ID] = c
	}
	ordered := make(CommentList, 0, len(junctions))
	for _, j := range junctions {
		if c, ok := byID[j.CommentID]; ok {
			// Ensure location fields match the junction (source of truth).
			c.CommitSHA = j.CommitSHA
			c.TreePath = j.TreePath
			c.Line = j.Line
			ordered = append(ordered, c)
		}
	}

	if err := ordered.LoadPosters(ctx); err != nil {
		return nil, err
	}
	if err := ordered.LoadAttachments(ctx); err != nil {
		return nil, err
	}
	return ordered, nil
}

// FindCommitCommentsByCommitSHA returns all inline comments for a commit.
func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) (CommentList, error) {
	return findCommitComments(ctx, builder.Eq{"repo_id": repoID, "commit_sha": commitSHA})
}

// FindCommitCommentsByLine returns comments anchored on one diff coordinate.
func FindCommitCommentsByLine(ctx context.Context, repoID int64, commitSHA, treePath string, line int64) (CommentList, error) {
	return findCommitComments(ctx, builder.Eq{
		"repo_id":    repoID,
		"commit_sha": commitSHA,
		"tree_path":  treePath,
		"line":       line,
	})
}

// FindCommitCommentsForFile returns comments for one file of a commit.
func FindCommitCommentsForFile(ctx context.Context, repoID int64, commitSHA, treePath string) (CommentList, error) {
	return findCommitComments(ctx, builder.Eq{
		"repo_id":    repoID,
		"commit_sha": commitSHA,
		"tree_path":  treePath,
	})
}

// DeleteCommitCommentsByRepoID removes all commit comments (junction + Comment + attachments) for a repo.
func DeleteCommitCommentsByRepoID(ctx context.Context, repoID int64) error {
	var junctions []*CommitComment
	if err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Find(&junctions); err != nil {
		return err
	}
	if len(junctions) == 0 {
		return nil
	}
	commentIDs := container.FilterSlice(junctions, func(j *CommitComment) (int64, bool) {
		return j.CommentID, j.CommentID > 0
	})
	if _, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(&CommitComment{}); err != nil {
		return err
	}
	for _, id := range commentIDs {
		if _, err := repo_model.DeleteAttachmentsByComment(ctx, id, true); err != nil {
			return err
		}
	}
	_, err := db.GetEngine(ctx).In("id", commentIDs).And(builder.Eq{"type": CommentTypeCommitComment}).Delete(&Comment{})
	return err
}
