// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

func ChangeActionsRawConcurrencyToText(ctx context.Context, x base.EngineMigration) error {
	if x.Dialect().URI().DBType == schemas.SQLITE { // SQLite stores VARCHAR as TEXT already
		return nil
	}
	for _, table := range []string{"action_run", "action_run_job"} {
		if err := base.ModifyColumn(ctx, x, table, &schemas.Column{
			Name:           "raw_concurrency",
			SQLType:        schemas.SQLType{Name: schemas.Text},
			Nullable:       true,
			DefaultIsEmpty: true,
		}); err != nil {
			return fmt.Errorf("ChangeActionsRawConcurrencyToText: %s: %w", table, err)
		}
	}
	return nil
}
