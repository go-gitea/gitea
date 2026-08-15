// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
)

// DropLeftoverActionRunConcurrencyColumns re-runs the "action_run" column drop of migration 331.
// On SQLite that drop used to be a no-op whenever the stored schema text did not quote identifiers
// with backticks, leaving a NOT NULL column that no model writes, so every run insert failed.
func DropLeftoverActionRunConcurrencyColumns(ctx context.Context, x base.EngineMigration) error {
	leftover := make([]string, 0, 2)
	for _, col := range []string{"concurrency_group", "concurrency_cancel"} {
		exist, err := x.Dialect().IsColumnExist(x.DB(), ctx, "action_run", col)
		if err != nil {
			return err
		}
		if exist {
			leftover = append(leftover, col)
		}
	}
	if len(leftover) == 0 {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "action_run", leftover...); err != nil {
		return err
	}
	return sess.Commit()
}
