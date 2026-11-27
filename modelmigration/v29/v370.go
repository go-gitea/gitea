// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/setting"
)

func ChangeActionsRawConcurrencyToText(_ context.Context, x base.EngineMigration) error {
	if !setting.Database.Type.IsMSSQL() { // startup sync widens VARCHAR to TEXT on MySQL and PostgreSQL
		return nil
	}
	for _, table := range []string{"action_run", "action_run_job"} {
		if _, err := x.Exec("ALTER TABLE " + table + " ALTER COLUMN raw_concurrency NVARCHAR(MAX) NULL"); err != nil {
			return fmt.Errorf("ChangeActionsRawConcurrencyToText: %s: %w", table, err)
		}
	}
	return nil
}
