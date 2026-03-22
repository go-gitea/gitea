// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"html/template"
	"strconv"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// CommitCodeComment represents an inline comment on a commit diff
type CommitCodeComment struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"INDEX"`
	Repo        *Repository        `xorm:"-"`
	CommitSHA   string             `xorm:"VARCHAR(64) INDEX"`
	PosterID    int64              `xorm:"INDEX"`
	Poster      *user_model.User   `xorm:"-"`
	TreePath    string             `xorm:"VARCHAR(4000)"`
	Line        int64              // negative = old/left side, positive = new/right side
	Content     string             `xorm:"LONGTEXT"`
	Patch       string             `xorm:"LONGTEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	RenderedContent template.HTML `xorm:"-"`
}

func init() {
	db.RegisterModel(new(CommitCodeComment))
}

// UnsignedLine returns the absolute value of the line number
func (c *CommitCodeComment) UnsignedLine() uint64 {
	if c.Line < 0 {
		return uint64(-c.Line)
	}
	return uint64(c.Line)
}

// DiffSide returns "previous" for left-side comments, "proposed" for right-side
func (c *CommitCodeComment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// HashTag returns the unique HTML tag ID for this comment
func (c *CommitCodeComment) HashTag() string {
	return "commitcomment-" + strconv.FormatInt(c.ID, 10)
}

// LoadPoster loads the poster user
func (c *CommitCodeComment) LoadPoster(ctx context.Context) error {
	if c.Poster != nil || c.PosterID == 0 {
		return nil
	}
	var err error
	c.Poster, err = user_model.GetUserByID(ctx, c.PosterID)
	return err
}

// CommitCodeComments maps file path -> line number -> list of comments
type CommitCodeComments map[string]map[int64][]*CommitCodeComment

// FetchCommitCodeComments returns all inline comments for a given repo commit (without rendering)
func FetchCommitCodeComments(ctx context.Context, repo *Repository, commitSHA string) (CommitCodeComments, error) {
	comments := make([]*CommitCodeComment, 0, 10)
	if err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": repo.ID, "commit_sha": commitSHA}).
		Asc("created_unix").
		Asc("id").
		Find(&comments); err != nil {
		return nil, err
	}

	for _, c := range comments {
		if err := c.LoadPoster(ctx); err != nil {
			return nil, err
		}
	}

	result := make(CommitCodeComments)
	for _, c := range comments {
		if result[c.TreePath] == nil {
			result[c.TreePath] = make(map[int64][]*CommitCodeComment)
		}
		result[c.TreePath][c.Line] = append(result[c.TreePath][c.Line], c)
	}
	return result, nil
}

// FetchCommitCodeCommentsByLine returns comments for a specific file path and line
func FetchCommitCodeCommentsByLine(ctx context.Context, repoID int64, commitSHA, treePath string, line int64) ([]*CommitCodeComment, error) {
	comments := make([]*CommitCodeComment, 0, 5)
	if err := db.GetEngine(ctx).
		Where(builder.Eq{
			"repo_id":    repoID,
			"commit_sha": commitSHA,
			"tree_path":  treePath,
			"line":       line,
		}).
		Asc("created_unix").
		Asc("id").
		Find(&comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// CreateCommitCodeComment inserts a new commit code comment
func CreateCommitCodeComment(ctx context.Context, opts *CreateCommitCodeCommentOptions) (*CommitCodeComment, error) {
	c := &CommitCodeComment{
		RepoID:    opts.Repo.ID,
		CommitSHA: opts.CommitSHA,
		PosterID:  opts.Doer.ID,
		Poster:    opts.Doer,
		TreePath:  opts.TreePath,
		Line:      opts.Line,
		Content:   opts.Content,
		Patch:     opts.Patch,
	}
	return c, db.Insert(ctx, c)
}

// CreateCommitCodeCommentOptions holds the options for creating a commit code comment
type CreateCommitCodeCommentOptions struct {
	Repo      *Repository
	Doer      *user_model.User
	CommitSHA string
	TreePath  string
	Line      int64
	Content   string
	Patch     string
}

// DeleteCommitCodeComment deletes a commit code comment by ID
func DeleteCommitCodeComment(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(new(CommitCodeComment))
	return err
}
