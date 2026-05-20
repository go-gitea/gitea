// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
)

func AddActionRunJobSummaryTable(x db.EngineMigration) error {
	return x.Sync(new(actions.ActionRunJobSummary))
}
