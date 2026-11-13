// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/setting"
)

// DropLegacyActionRunConcurrencyColumns retries migration 331's incomplete SQLite cleanup.
func DropLegacyActionRunConcurrencyColumns(_ctx context.Context, x base.EngineMigration) error {
	if !setting.Database.Type.IsSQLite3() {
		return nil
	}

	return base.EngineTransaction(x, func(sess base.Session) error {
		return base.DropTableColumns(sess, "action_run", "concurrency_group", "concurrency_cancel")
	})
}
