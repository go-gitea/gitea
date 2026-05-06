// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

type mirrorWithLastMirrorSyncUnix struct {
	LastMirrorSyncUnix int64 `xorm:"INDEX"`
}

func (mirrorWithLastMirrorSyncUnix) TableName() string {
	return "mirror"
}

func AddLastMirrorSyncUnixToMirror(x *xorm.Engine) error {
	if err := x.Sync(new(mirrorWithLastMirrorSyncUnix)); err != nil {
		return err
	}
	return nil
}
