// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
)

// DropLegacyActionRunConcurrencyColumns re-runs the column drop of migration 331, which used to be a no-op on SQLite
// when the stored schema text quoted identifiers differently, leaving NOT NULL columns that no model writes any more
func DropLegacyActionRunConcurrencyColumns(ctx context.Context, x base.EngineMigration) error {
	leftover := make([]string, 0, 2)
	for _, name := range []string{"concurrency_group", "concurrency_cancel"} {
		exist, err := x.Dialect().IsColumnExist(x.DB(), ctx, "action_run", name)
		if err != nil {
			return err
		}
		if exist {
			leftover = append(leftover, name)
		}
	}
	if len(leftover) == 0 {
		return nil
	}

	return base.EngineTransaction(x, func(sess base.Session) error {
		return base.DropTableColumns(sess, "action_run", leftover...)
	})
}
