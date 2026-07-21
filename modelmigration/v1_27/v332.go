// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

type mirrorWithLastSyncUnix struct {
	LastSyncUnix int64 `xorm:"INDEX"`
}

func (mirrorWithLastSyncUnix) TableName() string {
	return "mirror"
}

func AddLastSyncUnixToMirror(x base.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(mirrorWithLastSyncUnix))
	return err
}
