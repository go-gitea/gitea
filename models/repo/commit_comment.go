// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"
	"fmt"
	"html/template"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

// ErrInvalidCommitCommentLine is returned when the comment line is zero. Diff
// line numbers are signed (negative = old side, positive = new side) so zero
// is never a valid value.
var ErrInvalidCommitCommentLine = errors.New("commit comment line must be non-zero")

// CommitComment is an inline comment on a commit diff. It is intentionally
// a standalone model with no relation to the Issue/PR Comment system: there
// is no edit history, no reactions, no attachments, and no review threading.
type CommitComment struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"INDEX NOT NULL"`
	CommitSHA   string             `xorm:"VARCHAR(64) INDEX NOT NULL"`
	TreePath    string             `xorm:"VARCHAR(4000) NOT NULL"`
	Line        int64              `xorm:"NOT NULL"`
	PosterID    int64              `xorm:"INDEX NOT NULL"`
	Content     string             `xorm:"LONGTEXT NOT NULL"`
	Patch       string             `xorm:"LONGTEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	Poster          *user_model.User `xorm:"-"`
	RenderedContent template.HTML    `xorm:"-"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

// CommitCommentHashTag returns the fragment identifier of a commit comment,
// callable where only the id is at hand (e.g. a notification row).
func CommitCommentHashTag(id int64) string {
	return fmt.Sprintf("commitcomment-%d", id)
}

// HashTag returns the fragment identifier for templates (e.g. "commitcomment-42").
func (c *CommitComment) HashTag() string {
	return CommitCommentHashTag(c.ID)
}

// DiffSide returns "previous" if the comment is on the old side of a diff,
// "proposed" otherwise. Matches the convention used by review comments.
func (c *CommitComment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// UnsignedLine returns the absolute line number for template rendering.
func (c *CommitComment) UnsignedLine() uint64 {
	if c.Line < 0 {
		return uint64(-c.Line)
	}
	return uint64(c.Line)
}

// CommitCommentList is a slice of *CommitComment with helpers that mirror
// the (very small) parts of CommentList that templates rely on.
type CommitCommentList []*CommitComment

// LoadPosters loads each comment's Poster in a single bulk query.
func (cl CommitCommentList) LoadPosters(ctx context.Context) error {
	if len(cl) == 0 {
		return nil
	}

	posterIDs := container.FilterSlice(cl, func(c *CommitComment) (int64, bool) {
		return c.PosterID, c.Poster == nil && c.PosterID > 0
	})
	posterMap, err := user_model.GetUsersMapByIDs(ctx, posterIDs)
	if err != nil {
		return err
	}

	for _, c := range cl {
		if c.Poster == nil {
			c.Poster = user_model.GetPossibleUserFromMap(c.PosterID, posterMap)
		}
	}
	return nil
}

func findCommitComments(ctx context.Context, cond builder.Cond) (CommitCommentList, error) {
	comments := make(CommitCommentList, 0)
	if err := db.GetEngine(ctx).
		Where(cond).
		OrderBy("created_unix ASC, id ASC").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}
	return comments, nil
}

// FindCommitCommentsByCommitSHA returns all comments for a given commit in
// a repo, ordered oldest-first, with Posters preloaded.
func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) (CommitCommentList, error) {
	return findCommitComments(ctx, builder.Eq{"repo_id": repoID, "commit_sha": commitSHA})
}

// FindCommitCommentsByLine returns every comment anchored on one diff
// coordinate, ordered oldest-first, with Posters preloaded.
func FindCommitCommentsByLine(ctx context.Context, repoID int64, commitSHA, treePath string, line int64) (CommitCommentList, error) {
	return findCommitComments(ctx, builder.Eq{"repo_id": repoID, "commit_sha": commitSHA, "tree_path": treePath, "line": line})
}

// FindCommitCommentsForFile returns the comments of a single file in a commit,
// ordered oldest-first, with Posters preloaded.
func FindCommitCommentsForFile(ctx context.Context, repoID int64, commitSHA, treePath string) (CommitCommentList, error) {
	return findCommitComments(ctx, builder.Eq{"repo_id": repoID, "commit_sha": commitSHA, "tree_path": treePath})
}

// FindCommitCommentRepoIDs maps the given comment ids to the repo they belong
// to, omitting ids that no longer exist. It reads no body columns: callers only
// need to know whether a stored reference is still valid.
func FindCommitCommentRepoIDs(ctx context.Context, ids []int64) (map[int64]int64, error) {
	repoIDs := make(map[int64]int64, len(ids))
	for len(ids) > 0 {
		limit := min(len(ids), db.DefaultMaxInSize)
		var batch []*CommitComment
		if err := db.GetEngine(ctx).Cols("id", "repo_id").In("id", ids[:limit]).Find(&batch); err != nil {
			return nil, err
		}
		for _, c := range batch {
			repoIDs[c.ID] = c.RepoID
		}
		ids = ids[limit:]
	}
	return repoIDs, nil
}

// GetCommitCommentPosterIDs returns the distinct posters of every comment on a
// commit, used to notify the participants of an ongoing conversation.
func GetCommitCommentPosterIDs(ctx context.Context, repoID int64, commitSHA string) ([]int64, error) {
	ids := make([]int64, 0, 8)
	return ids, db.GetEngine(ctx).
		Table("commit_comment").
		Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		Select("poster_id").
		Distinct("poster_id").
		Find(&ids)
}

// CreateCommitComment inserts a new commit comment. Line=0 is rejected
// because diff coordinates are signed and zero has no diff-side meaning.
func CreateCommitComment(ctx context.Context, c *CommitComment) error {
	if c.Line == 0 {
		return ErrInvalidCommitCommentLine
	}
	_, err := db.GetEngine(ctx).Insert(c)
	return err
}

// DeleteCommitComment removes a commit comment by id, scoped to the repo.
func DeleteCommitComment(ctx context.Context, repoID, id int64) error {
	_, err := db.GetEngine(ctx).
		Where("repo_id = ? AND id = ?", repoID, id).
		Delete(&CommitComment{})
	return err
}

// GetCommitCommentByID returns a commit comment by id, scoped to the repo.
func GetCommitCommentByID(ctx context.Context, repoID, id int64) (*CommitComment, error) {
	c := &CommitComment{}
	has, err := db.GetEngine(ctx).
		Where("repo_id = ? AND id = ?", repoID, id).
		Get(c)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{Resource: "CommitComment", ID: id}
	}
	return c, nil
}
