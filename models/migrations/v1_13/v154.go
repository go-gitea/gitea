// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddTimeStamps(x *xorm.Engine) error {
	// this will add timestamps where it is useful to have

	// Star represents a starred repo by an user.
	type Star struct {
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	}
	if err := x.Sync(new(Star)); err != nil {
		return err
	}

	// Label represents a label of repository for issues.
	type Label struct {
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	if err := x.Sync(new(Label)); err != nil {
		return err
	}

	// Follow represents relations of user and their followers.
	type Follow struct {
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	}
	if err := x.Sync(new(Follow)); err != nil {
		return err
	}

	// Watch is connection request for receiving repository notification.
	type Watch struct {
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	if err := x.Sync(new(Watch)); err != nil {
		return err
	}

	// Collaboration represent the relation between an individual and a repository.
	type Collaboration struct {
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	return x.Sync(new(Collaboration))
}
