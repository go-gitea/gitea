// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import "code.gitea.io/gitea/models/db"


func RecreateUserTableToFixDefaultValues(_ db.EngineMigration) error {
	return nil
}
