// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package versioned_migration

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/models/migrations"
	"gitea.dev/modules/globallock"
)

func Migrate(ctx context.Context, x db.EngineMigration) error {
	// only one instance can do the migration at the same time if there are multiple instances
	release, err := globallock.Lock(ctx, "gitea_versioned_migration")
	if err != nil {
		return err
	}
	defer release()

	return migrations.Migrate(ctx, x)
}
