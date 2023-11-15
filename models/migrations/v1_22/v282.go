// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

func AddIndexToPullAutoMergeDoerID(x *xorm.Engine) error {
	type PullAutoMerge struct {
		DoerID int64 `xorm:"INDEX NOT NULL"`
	}

	return x.Sync(&PullAutoMerge{})
}
