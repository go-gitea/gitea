// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import "gitea.dev/modelmigration/base"

func DropTableRemoteVersion(x base.EngineMigration) error {
	// drop the orphaned table introduced in `v199`, now the update checker also uses AppState, do not need this table
	_ = x.DropTables("remote_version")
	return nil
}
