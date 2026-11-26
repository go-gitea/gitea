// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"fmt"
	"html/template"
	"time"

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

type Comment struct {
	ID              int64              `xorm:"pk autoincr"`
	AdvisoryID      int64              `xorm:"INDEX NOT NULL"`
	PosterID        int64              `xorm:"INDEX NOT NULL"`
	Poster          *user_model.User   `xorm:"-"`
	Content         string             `xorm:"LONGTEXT"`
	IsConfidential  bool               `xorm:"NOT NULL DEFAULT false"`
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
	AdvisoryID          int64
	IncludeConfidential bool
	UpdatedAfter        time.Time
}

func commentConds(advisoryID int64, includeConfidential bool) builder.Cond {
	cond := builder.Eq{"advisory_id": advisoryID}
	if !includeConfidential {
		cond["is_confidential"] = false
	}
	return cond
}

func (opts FindCommentsOptions) ToConds() builder.Cond {
	cond := commentConds(opts.AdvisoryID, opts.IncludeConfidential)
	if !opts.UpdatedAfter.IsZero() {
		cond = cond.And(builder.Gt{"updated_unix": opts.UpdatedAfter.Unix()})
	}
	return cond
}

func (opts FindCommentsOptions) ToOrders() string {
	return "created_unix, id"
}

func LoadCommentPosters(ctx context.Context, comments []*Comment) error {
	posters, err := user_model.GetUsersMapByIDs(ctx, container.FilterSlice(comments, func(comment *Comment) (int64, bool) { return comment.PosterID, true }))
	if err != nil {
		return err
	}
	for _, comment := range comments {
		comment.Poster = user_model.GetPossibleUserFromMap(comment.PosterID, posters)
	}
	return nil
}

func GetCommentByID(ctx context.Context, advisoryID, id int64, includeConfidential bool) (*Comment, error) {
	comment, found, err := db.Get[Comment](ctx, commentConds(advisoryID, includeConfidential).And(builder.Eq{"id": id}))
	if err != nil {
		return nil, err
	} else if !found {
		return nil, ErrCommentNotExist{ID: id}
	}
	return comment, nil
}

func UpdateCommentContent(ctx context.Context, comment *Comment) error {
	_, err := db.GetEngine(ctx).ID(comment.ID).Cols("content").Update(comment)
	return err
}
