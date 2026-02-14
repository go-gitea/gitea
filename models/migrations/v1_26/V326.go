// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func AddTokenPermissionsToActionRunJob(x *xorm.Engine) error {
	type ActionRunJob struct {
		TokenPermissions string `xorm:"TEXT"`
	}
	return x.SyncWithOptions(&xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob))
}
