// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"fmt"
	"html/template"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type ErrCommentNotExist struct {
	ID int64
}

func (err ErrCommentNotExist) Error() string {
	return fmt.Sprintf("security advisory comment does not exist [id: %d]", err.ID)
}

func (err ErrCommentNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Comment belongs to the private discussion of an advisory
type Comment struct {
	ID              int64              `xorm:"pk autoincr"`
	AdvisoryID      int64              `xorm:"INDEX NOT NULL"`
	PosterID        int64              `xorm:"INDEX NOT NULL"`
	Poster          *user_model.User   `xorm:"-"`
	Content         string             `xorm:"LONGTEXT"`
	IsInternal      bool               `xorm:"NOT NULL DEFAULT false"` // hidden from the reporter and read-only collaborators
	RenderedContent template.HTML      `xorm:"-"`
	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(Comment))
}

func (Comment) TableName() string {
	return "security_advisory_comment"
}

type FindCommentsOptions struct {
	db.ListOptions
	AdvisoryID      int64
	IncludeInternal bool
}

func commentConds(advisoryID int64, includeInternal bool) builder.Cond {
	cond := builder.Eq{"advisory_id": advisoryID}
	if !includeInternal {
		cond["is_internal"] = false
	}
	return cond
}

func (opts FindCommentsOptions) ToConds() builder.Cond {
	return commentConds(opts.AdvisoryID, opts.IncludeInternal)
}

func (opts FindCommentsOptions) ToOrders() string {
	return "created_unix, id"
}

type CommentList []*Comment

func (comments CommentList) LoadPosters(ctx context.Context) error {
	posters, err := user_model.GetUsersMapByIDs(ctx, container.FilterSlice(comments, func(c *Comment) (int64, bool) { return c.PosterID, true }))
	if err != nil {
		return err
	}
	for _, c := range comments {
		c.Poster = user_model.GetPossibleUserFromMap(c.PosterID, posters)
	}
	return nil
}

func GetCommentByID(ctx context.Context, advisoryID, id int64, includeInternal bool) (*Comment, error) {
	c, has, err := db.Get[Comment](ctx, commentConds(advisoryID, includeInternal).And(builder.Eq{"id": id}))
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{ID: id}
	}
	return c, nil
}

func CreateComment(ctx context.Context, c *Comment) error {
	return db.Insert(ctx, c)
}

func UpdateCommentContent(ctx context.Context, c *Comment) error {
	_, err := db.GetEngine(ctx).ID(c.ID).Cols("content").Update(c)
	return err
}

func DeleteComment(ctx context.Context, c *Comment) error {
	_, err := db.DeleteByID[Comment](ctx, c.ID)
	return err
}
