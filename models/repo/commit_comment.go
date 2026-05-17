// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"
	"fmt"
	"html/template"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
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

// HashTag returns the fragment identifier for templates (e.g. "commitcomment-42").
func (c *CommitComment) HashTag() string {
	return fmt.Sprintf("commitcomment-%d", c.ID)
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

// LoadPoster fetches the comment's poster from the user table, idempotent.
func (c *CommitComment) LoadPoster(ctx context.Context) error {
	if c.Poster != nil {
		return nil
	}
	var err error
	c.PosterID, c.Poster, err = user_model.GetPossibleUserByID(ctx, c.PosterID)
	return err
}

// CommitCommentList is a slice of *CommitComment with helpers that mirror
// the (very small) parts of CommentList that templates rely on.
type CommitCommentList []*CommitComment

// LoadPosters loads each comment's Poster in a single bulk query.
func (cl CommitCommentList) LoadPosters(ctx context.Context) error {
	if len(cl) == 0 {
		return nil
	}
	posterIDs := make([]int64, 0, len(cl))
	seen := make(map[int64]struct{})
	for _, c := range cl {
		if _, ok := seen[c.PosterID]; ok {
			continue
		}
		seen[c.PosterID] = struct{}{}
		posterIDs = append(posterIDs, c.PosterID)
	}

	posterMap, err := user_model.GetUsersMapByIDs(ctx, posterIDs)
	if err != nil {
		return err
	}

	for _, c := range cl {
		if p, ok := posterMap[c.PosterID]; ok {
			c.Poster = p
		} else {
			c.Poster = user_model.NewGhostUser()
		}
	}
	return nil
}

// FileCommitComments holds commit comments for a single file, split by side
// (left = old, right = new) with int keys matching DiffLine indices.
type FileCommitComments struct {
	Left  map[int][]*CommitComment
	Right map[int][]*CommitComment
}

// CommitCommentsForDiff maps file paths to their commit comments.
type CommitCommentsForDiff map[string]*FileCommitComments

// FindCommitCommentsByCommitSHA returns all comments for a given commit in
// a repo, ordered oldest-first, with Posters preloaded.
func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) (CommitCommentList, error) {
	comments := make(CommitCommentList, 0)
	if err := db.GetEngine(ctx).
		Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		OrderBy("created_unix ASC").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}
	return comments, nil
}

// FindCommitCommentsForDiff returns comments grouped by path and side for
// rendering in a diff view.
func FindCommitCommentsForDiff(ctx context.Context, repoID int64, commitSHA string) (CommitCommentsForDiff, error) {
	comments, err := FindCommitCommentsByCommitSHA(ctx, repoID, commitSHA)
	if err != nil {
		return nil, err
	}

	result := make(CommitCommentsForDiff)
	for _, c := range comments {
		fcc, ok := result[c.TreePath]
		if !ok {
			fcc = &FileCommitComments{
				Left:  make(map[int][]*CommitComment),
				Right: make(map[int][]*CommitComment),
			}
			result[c.TreePath] = fcc
		}
		if c.Line < 0 {
			idx := int(-c.Line)
			fcc.Left[idx] = append(fcc.Left[idx], c)
		} else {
			idx := int(c.Line)
			fcc.Right[idx] = append(fcc.Right[idx], c)
		}
	}
	return result, nil
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
