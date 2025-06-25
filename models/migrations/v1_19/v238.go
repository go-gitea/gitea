// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// AddUpdatedUnixToLFSMetaObject adds an updated column to the LFSMetaObject to allow for garbage collection
func AddUpdatedUnixToLFSMetaObject(x *xorm.Engine) error {
	// Drop the table introduced in `v211`, it's considered badly designed and doesn't look like to be used.
	// See: https://github.com/go-gitea/gitea/issues/21086#issuecomment-1318217453
	// LFSMetaObject stores metadata for LFS tracked files.
	type LFSMetaObject struct {
		ID           int64              `xorm:"pk autoincr"`
		Oid          string             `json:"oid" xorm:"UNIQUE(s) INDEX NOT NULL"`
		Size         int64              `json:"size" xorm:"NOT NULL"`
		RepositoryID int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CreatedUnix  timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix  timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync(new(LFSMetaObject))
}
