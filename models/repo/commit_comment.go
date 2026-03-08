// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"html/template"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
)

// CommitComment represents an inline comment on a commit diff.
type CommitComment struct {
	ID              int64              `xorm:"pk autoincr"`
	RepoID          int64              `xorm:"INDEX NOT NULL"`
	CommitSHA       string             `xorm:"VARCHAR(64) INDEX NOT NULL"`
	TreePath        string             `xorm:"VARCHAR(4000) NOT NULL"`
	Line            int64              `xorm:"NOT NULL"` // negative = old side, positive = new side
	PosterID        int64              `xorm:"INDEX NOT NULL"`
	Poster          *user_model.User   `xorm:"-"`
	Content         string             `xorm:"LONGTEXT NOT NULL"`
	RenderedContent template.HTML      `xorm:"-"`
	Patch           string             `xorm:"LONGTEXT"`
	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

// HashTag returns a unique tag for the comment, used for anchoring.
func (c *CommitComment) HashTag() string {
	return fmt.Sprintf("commitcomment-%d", c.ID)
}

// UnsignedLine returns the absolute value of the line number.
func (c *CommitComment) UnsignedLine() int64 {
	if c.Line < 0 {
		return -c.Line
	}
	return c.Line
}

// GetCommentSide returns "previous" for old side (negative Line), "proposed" for new side.
func (c *CommitComment) GetCommentSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// LoadPoster loads the poster user for a commit comment.
func (c *CommitComment) LoadPoster(ctx context.Context) error {
	if c.Poster != nil || c.PosterID <= 0 {
		return nil
	}
	poster, err := user_model.GetUserByID(ctx, c.PosterID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			c.PosterID = user_model.GhostUserID
			c.Poster = user_model.NewGhostUser()
			return nil
		}
		return err
	}
	c.Poster = poster
	return nil
}

// FileCommitComments holds commit comments for a single file,
// split by side (left = old, right = new) with int keys matching DiffLine indices.
type FileCommitComments struct {
	Left  map[int][]*CommitComment
	Right map[int][]*CommitComment
}

// CommitCommentsForDiff maps file paths to their commit comments.
type CommitCommentsForDiff map[string]*FileCommitComments

// FindCommitCommentsByCommitSHA returns all comments for a given commit in a repo.
func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) ([]*CommitComment, error) {
	comments := make([]*CommitComment, 0, 10)
	return comments, db.GetEngine(ctx).
		Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		OrderBy("created_unix ASC").
		Find(&comments)
}

// FindCommitCommentsForDiff returns comments grouped by path and side for rendering in a diff view.
func FindCommitCommentsForDiff(ctx context.Context, repoID int64, commitSHA string) (CommitCommentsForDiff, error) {
	comments, err := FindCommitCommentsByCommitSHA(ctx, repoID, commitSHA)
	if err != nil {
		return nil, err
	}

	result := make(CommitCommentsForDiff)
	for _, c := range comments {
		if err := c.LoadPoster(ctx); err != nil {
			return nil, err
		}
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

// CreateCommitComment inserts a new commit comment.
func CreateCommitComment(ctx context.Context, c *CommitComment) error {
	_, err := db.GetEngine(ctx).Insert(c)
	return err
}

// GetCommitCommentByID returns a commit comment by its ID.
func GetCommitCommentByID(ctx context.Context, id int64) (*CommitComment, error) {
	c := &CommitComment{}
	has, err := db.GetEngine(ctx).ID(id).Get(c)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{Resource: "CommitComment", ID: id}
	}
	return c, nil
}

// DeleteCommitComment deletes a commit comment by ID.
func DeleteCommitComment(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&CommitComment{})
	return err
}

// CountCommitCommentsByCommitSHA returns the count of comments for a commit.
func CountCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) (int64, error) {
	return db.GetEngine(ctx).
		Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		Count(&CommitComment{})
}
