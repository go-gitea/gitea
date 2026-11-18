// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

// ErrCommitCommentNotExist represents a "CommitCommentNotExist" kind of error.
var ErrCommitCommentNotExist = util.NewNotExistErrorf("commit comment does not exist")

// ErrCommitCommentAlreadyChanged is returned when the comment content was changed by someone else.
var ErrCommitCommentAlreadyChanged = util.NewInvalidArgumentErrorf("the comment is already changed")

// CommitComment is a comment on a line of a commit's diff.
type CommitComment struct {
	ID        int64            `xorm:"pk autoincr"`
	RepoID    int64            `xorm:"INDEX NOT NULL"`
	CommitSHA string           `xorm:"VARCHAR(64) INDEX NOT NULL"`
	PosterID  int64            `xorm:"INDEX NOT NULL"`
	Poster    *user_model.User `xorm:"-"`

	TreePath string `xorm:"VARCHAR(4000)"` // SQLServer only supports up to 4000
	Line     int64  // negative: previous (left) side, positive: proposed (right) side

	Content        string `xorm:"LONGTEXT"`
	ContentVersion int    `xorm:"NOT NULL DEFAULT 0"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	RenderedContent template.HTML            `xorm:"-"`
	Attachments     []*repo_model.Attachment `xorm:"-"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

// HashTag returns the anchor of the comment.
func (c *CommitComment) HashTag() string {
	return fmt.Sprintf("commitcomment-%d", c.ID)
}

// DiffSide returns "previous" or "proposed".
func (c *CommitComment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// UnsignedLine returns the line number without the side sign.
func (c *CommitComment) UnsignedLine() int64 {
	return max(c.Line, -c.Line)
}

// LoadPoster loads the poster of the comment.
func (c *CommitComment) LoadPoster(ctx context.Context) (err error) {
	if c.Poster != nil {
		return nil
	}
	_, c.Poster, err = user_model.GetPossibleUserByID(ctx, c.PosterID)
	return err
}

// LoadAttachments loads the attachments of the comment.
func (c *CommitComment) LoadAttachments(ctx context.Context) (err error) {
	if len(c.Attachments) > 0 {
		return nil
	}
	c.Attachments, err = repo_model.GetAttachmentsByCommitCommentID(ctx, c.ID)
	return err
}

func sanitizeCommitCommentContent(s string) string {
	// postgresql doesn't like "\x00"
	return strings.ReplaceAll(s, "\x00", " ")
}

// attachToCommitComment links the uploaded attachments to the comment.
func attachToCommitComment(ctx context.Context, c *CommitComment, uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}
	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, uuids)
	if err != nil {
		return fmt.Errorf("attachToCommitComment: %w", err)
	}
	for _, a := range attachments {
		if a.CommitCommentID == c.ID {
			continue
		}
		if a.RepoID != c.RepoID {
			return util.NewPermissionDeniedErrorf("attachment belongs to a different repository")
		}
		if a.IsLinked() {
			return util.NewPermissionDeniedErrorf("attachment is already linked to something else")
		}
		a.CommitCommentID = c.ID
		if err := repo_model.UpdateAttachment(ctx, a); err != nil {
			return fmt.Errorf("attachToCommitComment: %w", err)
		}
		c.Attachments = append(c.Attachments, a)
	}
	return nil
}

// CreateCommitComment creates a comment and links the attachments (by UUID) to it.
func CreateCommitComment(ctx context.Context, c *CommitComment, attachmentUUIDs []string) error {
	c.Content = sanitizeCommitCommentContent(c.Content)
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.Insert(ctx, c); err != nil {
			return err
		}
		return attachToCommitComment(ctx, c, attachmentUUIDs)
	})
}

// GetCommitCommentByID returns the comment with the given ID.
func GetCommitCommentByID(ctx context.Context, id int64) (*CommitComment, error) {
	c, exist, err := db.GetByID[CommitComment](ctx, id)
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrCommitCommentNotExist
	}
	return c, nil
}

// ListCommitComments returns all comments of the commit ordered by creation time.
func ListCommitComments(ctx context.Context, repoID int64, commitSHA string) ([]*CommitComment, error) {
	comments := make([]*CommitComment, 0, 10)
	return comments, db.GetEngine(ctx).Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		Asc("created_unix", "id").Find(&comments)
}

// LoadCommitCommentsAttachments loads the attachments of all given comments with one query.
func LoadCommitCommentsAttachments(ctx context.Context, comments []*CommitComment) error {
	if len(comments) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(comments))
	byID := make(map[int64]*CommitComment, len(comments))
	for _, c := range comments {
		ids = append(ids, c.ID)
		byID[c.ID] = c
	}
	attachments := make([]*repo_model.Attachment, 0, len(comments))
	if err := db.GetEngine(ctx).In("commit_comment_id", ids).Find(&attachments); err != nil {
		return err
	}
	for _, a := range attachments {
		byID[a.CommitCommentID].Attachments = append(byID[a.CommitCommentID].Attachments, a)
	}
	return nil
}

// UpdateCommitComment updates the content and the attachments of the comment.
// If syncAttachments is set, attachments which are not in attachmentUUIDs are deleted.
func UpdateCommitComment(ctx context.Context, c *CommitComment, contentVersion int, attachmentUUIDs []string, syncAttachments bool) error {
	c.Content = sanitizeCommitCommentContent(c.Content)
	return db.WithTx(ctx, func(ctx context.Context) error {
		c.ContentVersion = contentVersion + 1
		affected, err := db.GetEngine(ctx).ID(c.ID).Cols("content", "content_version").
			Where("content_version = ?", contentVersion).Update(c)
		if err != nil {
			return err
		} else if affected == 0 {
			return ErrCommitCommentAlreadyChanged
		}
		if !syncAttachments {
			return nil
		}
		if err := c.LoadAttachments(ctx); err != nil {
			return err
		}
		kept := make([]*repo_model.Attachment, 0, len(c.Attachments))
		for _, a := range c.Attachments {
			if util.SliceContainsString(attachmentUUIDs, a.UUID) {
				kept = append(kept, a)
				continue
			}
			if err := repo_model.DeleteAttachment(ctx, a, true); err != nil {
				return err
			}
		}
		c.Attachments = kept
		return attachToCommitComment(ctx, c, attachmentUUIDs)
	})
}

// DeleteCommitComment deletes the comment and its attachments.
func DeleteCommitComment(ctx context.Context, c *CommitComment) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := repo_model.DeleteAttachmentsByCommitComment(ctx, c.ID, true); err != nil {
			return err
		}
		_, err := db.GetEngine(ctx).ID(c.ID).Delete(new(CommitComment))
		return err
	})
}
