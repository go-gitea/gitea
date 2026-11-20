// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/setting"
)

func WidenCommitStatusState(_ context.Context, x base.EngineMigration) error {
	if !setting.Database.Type.IsMSSQL() { // startup sync widens VARCHAR on MySQL and PostgreSQL, SQLite doesn't enforce the length
		return nil
	}
	for _, table := range []string{"commit_status", "commit_status_summary"} {
		if _, err := x.Exec("ALTER TABLE " + table + " ALTER COLUMN state NVARCHAR(32) NOT NULL"); err != nil {
			return fmt.Errorf("WidenCommitStatusState: %s: %w", table, err)
		}
	}
	return nil
}
