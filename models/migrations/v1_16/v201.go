// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"xorm.io/xorm"
)

func DropTableRemoteVersion(x *xorm.Engine) error {
	// drop the orphaned table introduced in `v199`, now the update checker also uses AppState, do not need this table
	_ = x.DropTables("remote_version")
	return nil
}
