// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.gitea.io/gitea/models/db"
)

func DropWronglyCreatedTable(x db.EngineMigration) error {
	return x.DropTables("o_auth2_application")
}
