// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddIndexToPullAutoMergeDoerID(_ context.Context, x base.EngineMigration) error {
	type PullAutoMerge struct {
		DoerID int64 `xorm:"INDEX NOT NULL"`
	}

	return x.Sync(&PullAutoMerge{})
}
