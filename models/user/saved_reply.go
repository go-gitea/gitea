// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

var (
	ErrSavedReplyDoesNotBelongToUser = util.NewNotExistErrorf("saved reply does not belong to user")
	ErrSavedReplyContentEmpty        = util.NewInvalidArgumentErrorf("saved reply content cannot be empty")
	ErrSavedReplyTitleEmpty          = util.NewInvalidArgumentErrorf("saved reply title cannot be empty")
)

type SavedReply struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX NOT NULL"`

	Title   string `xorm:"VARCHAR(255) NOT NULL"`
	Content string `xorm:"TEXT NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func (*SavedReply) TableName() string {
	return "user_saved_replies"
}

func init() {
	db.RegisterModel(new(SavedReply))
}

type FindSavedReplyOptions struct {
	db.ListOptions
	UserID int64
	Title  string
}

func (opts *FindSavedReplyOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.UserID != 0 {
		cond = cond.And(builder.Eq{"user_saved_replies.user_id": opts.UserID})
	}
	if len(opts.Title) > 0 {
		cond = cond.And(db.BuildCaseInsensitiveLike("user_saved_replies.title", opts.Title))
	}
	return cond
}

func (opts *FindSavedReplyOptions) ToOrders() string {
	return "user_saved_replies.created_unix ASC"
}

func GetSavedReply(ctx context.Context, id int64) (*SavedReply, error) {
	savedReply := &SavedReply{}
	has, err := db.GetEngine(ctx).ID(id).Get(savedReply)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.NewNotExistErrorf("saved reply does not exist [id: %d]", id)
	}

	return savedReply, nil
}

func GetUserSavedReplies(ctx context.Context, userID int64, title string) ([]*SavedReply, error) {
	return db.Find[SavedReply](ctx, &FindSavedReplyOptions{
		UserID: userID,
		Title:  title,
	})
}

func UpdateSavedReply(ctx context.Context, id int64, title, content string) error {
	_, err := db.GetEngine(ctx).ID(id).Cols("title", "content").Update(&SavedReply{Title: title, Content: content})
	return err
}
