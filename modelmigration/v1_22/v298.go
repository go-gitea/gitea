// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import "gitea.dev/modelmigration/base"

func DropWronglyCreatedTable(x base.EngineMigration) error {
	return x.DropTables("o_auth2_application")
}
