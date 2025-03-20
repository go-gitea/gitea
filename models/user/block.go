// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

var (
	ErrBlockOrganization = util.NewInvalidArgumentErrorf("cannot block an organization")
	ErrCanNotBlock       = util.NewInvalidArgumentErrorf("cannot block the user")
	ErrCanNotUnblock     = util.NewInvalidArgumentErrorf("cannot unblock the user")
	ErrBlockedUser       = util.NewPermissionDeniedErrorf("user is blocked")
)

type Blocking struct {
	ID          int64 `xorm:"pk autoincr"`
	BlockerID   int64 `xorm:"UNIQUE(block)"`
	Blocker     *User `xorm:"-"`
	BlockeeID   int64 `xorm:"UNIQUE(block)"`
	Blockee     *User `xorm:"-"`
	Note        string
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

func (*Blocking) TableName() string {
	return "user_blocking"
}

func init() {
	db.RegisterModel(new(Blocking))
}

func UpdateBlockingNote(ctx context.Context, id int64, note string) error {
	_, err := db.GetEngine(ctx).ID(id).Cols("note").Update(&Blocking{Note: note})
	return err
}

func IsUserBlockedBy(ctx context.Context, blockee *User, blockerIDs ...int64) bool {
	if len(blockerIDs) == 0 {
		return false
	}

	if blockee.IsAdmin {
		return false
	}

	cond := builder.Eq{"user_blocking.blockee_id": blockee.ID}.
		And(builder.In("user_blocking.blocker_id", blockerIDs))

	has, _ := db.GetEngine(ctx).Where(cond).Exist(&Blocking{})
	return has
}

type FindBlockingOptions struct {
	db.ListOptions
	BlockerID int64
	BlockeeID int64
}

func (opts *FindBlockingOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.BlockerID != 0 {
		cond = cond.And(builder.Eq{"user_blocking.blocker_id": opts.BlockerID})
	}
	if opts.BlockeeID != 0 {
		cond = cond.And(builder.Eq{"user_blocking.blockee_id": opts.BlockeeID})
	}
	return cond
}

func FindBlockings(ctx context.Context, opts *FindBlockingOptions) ([]*Blocking, int64, error) {
	return db.FindAndCount[Blocking](ctx, opts)
}

func GetBlocking(ctx context.Context, blockerID, blockeeID int64) (*Blocking, error) {
	blocks, _, err := FindBlockings(ctx, &FindBlockingOptions{
		BlockerID: blockerID,
		BlockeeID: blockeeID,
	})
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, nil
	}
	return blocks[0], nil
}

type BlockingList []*Blocking

func (blocks BlockingList) LoadAttributes(ctx context.Context) error {
	ids := make(container.Set[int64], len(blocks)*2)
	for _, b := range blocks {
		ids.Add(b.BlockerID)
		ids.Add(b.BlockeeID)
	}

	userList, err := GetUsersByIDs(ctx, ids.Values())
	if err != nil {
		return err
	}

	userMap := make(map[int64]*User, len(userList))
	for _, u := range userList {
		userMap[u.ID] = u
	}

	for _, b := range blocks {
		b.Blocker = userMap[b.BlockerID]
		b.Blockee = userMap[b.BlockeeID]
	}

	return nil
}
