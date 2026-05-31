// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/actions"
	"gitea.dev/models/db"
)

func AddActionRunJobSummaryTable(x db.EngineMigration) error {
	return x.Sync(new(actions.ActionRunJobSummary))
}
