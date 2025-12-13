// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"html/template"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// CommitComment represents a comment on a specific line in a commit diff
type CommitComment struct {
	ID               int64            `xorm:"pk autoincr"`
	RepoID           int64            `xorm:"INDEX"`
	Repo             *Repository      `xorm:"-"`
	CommitSHA        string           `xorm:"VARCHAR(64) INDEX"`
	TreePath         string           `xorm:"VARCHAR(4000)"` // File path (same field name as issue comments)
	Line             int64            // - previous line / + proposed line
	Content          string           `xorm:"LONGTEXT"`
	ContentVersion   int              `xorm:"NOT NULL DEFAULT 0"`
	RenderedContent  template.HTML    `xorm:"-"`
	PosterID         int64            `xorm:"INDEX"`
	Poster           *user_model.User `xorm:"-"`
	OriginalAuthor   string
	OriginalAuthorID int64
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix      timeutil.TimeStamp `xorm:"INDEX updated"`
	Attachments      []*Attachment      `xorm:"-"`

	// Fields for template compatibility with PR comments
	Review      any  `xorm:"-"` // Always nil for commit comments
	Invalidated bool `xorm:"-"` // Always false for commit comments
	ResolveDoer any  `xorm:"-"` // Always nil for commit comments
	Reactions   any  `xorm:"-"` // Reactions for this comment
}

// IsResolved returns false (commit comments don't support resolution)
func (c *CommitComment) IsResolved() bool {
	return false
}

// HasOriginalAuthor returns if a comment was migrated and has an original author
func (c *CommitComment) HasOriginalAuthor() bool {
	return c.OriginalAuthor != "" && c.OriginalAuthorID != 0
}

func init() {
	db.RegisterModel(new(CommitComment))
}

// ErrCommitCommentNotExist represents a "CommitCommentNotExist" kind of error.
type ErrCommitCommentNotExist struct {
	ID int64
}

// IsErrCommitCommentNotExist checks if an error is a ErrCommitCommentNotExist.
func IsErrCommitCommentNotExist(err error) bool {
	_, ok := err.(ErrCommitCommentNotExist)
	return ok
}

func (err ErrCommitCommentNotExist) Error() string {
	return fmt.Sprintf("commit comment does not exist [id: %d]", err.ID)
}

// CreateCommitComment creates a new commit comment
func CreateCommitComment(ctx context.Context, comment *CommitComment) error {
	return db.Insert(ctx, comment)
}

// GetCommitCommentByID returns a commit comment by ID
func GetCommitCommentByID(ctx context.Context, id int64) (*CommitComment, error) {
	comment := new(CommitComment)
	has, err := db.GetEngine(ctx).ID(id).Get(comment)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommitCommentNotExist{id}
	}
	return comment, nil
}

// FindCommitCommentsOptions describes the conditions to find commit comments
type FindCommitCommentsOptions struct {
	db.ListOptions
	RepoID    int64
	CommitSHA string
	Path      string
	Line      int64
}

// ToConds implements FindOptions interface
func (opts FindCommitCommentsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.CommitSHA != "" {
		cond = cond.And(builder.Eq{"commit_sha": opts.CommitSHA})
	}
	if opts.Path != "" {
		cond = cond.And(builder.Eq{"tree_path": opts.Path})
	}
	if opts.Line != 0 {
		cond = cond.And(builder.Eq{"line": opts.Line})
	}
	return cond
}

// FindCommitComments returns commit comments based on options
func FindCommitComments(ctx context.Context, opts FindCommitCommentsOptions) ([]*CommitComment, error) {
	comments := make([]*CommitComment, 0, 10)
	sess := db.GetEngine(ctx).Where(opts.ToConds())
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, &opts)
	}
	return comments, sess.Find(&comments)
}

// LoadPoster loads the poster user
func (c *CommitComment) LoadPoster(ctx context.Context) error {
	if c.Poster != nil {
		return nil
	}
	var err error
	c.Poster, err = user_model.GetPossibleUserByID(ctx, c.PosterID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			c.PosterID = user_model.GhostUserID
			c.Poster = user_model.NewGhostUser()
		}
	}
	return err
}

// LoadRepo loads the repository
func (c *CommitComment) LoadRepo(ctx context.Context) error {
	if c.Repo != nil {
		return nil
	}
	var err error
	c.Repo, err = GetRepositoryByID(ctx, c.RepoID)
	return err
}

// LoadAttachments loads attachments
func (c *CommitComment) LoadAttachments(ctx context.Context) error {
	if len(c.Attachments) > 0 {
		return nil
	}
	var err error
	c.Attachments, err = GetAttachmentsByCommentID(ctx, c.ID)
	return err
}

// DiffSide returns "previous" if Line is negative and "proposed" if positive
func (c *CommitComment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// UnsignedLine returns the absolute value of the line number
func (c *CommitComment) UnsignedLine() uint64 {
	if c.Line < 0 {
		return uint64(c.Line * -1)
	}
	return uint64(c.Line)
}

// HashTag returns unique hash tag for comment
func (c *CommitComment) HashTag() string {
	return fmt.Sprintf("commitcomment-%d", c.ID)
}

// UpdateCommitComment updates a commit comment
func UpdateCommitComment(ctx context.Context, comment *CommitComment) error {
	_, err := db.GetEngine(ctx).ID(comment.ID).AllCols().Update(comment)
	return err
}

// DeleteCommitComment deletes a commit comment
func DeleteCommitComment(ctx context.Context, comment *CommitComment) error {
	_, err := db.GetEngine(ctx).ID(comment.ID).Delete(comment)
	return err
}
