// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package versioned_migration

import (
	"context"

	"codenest.dev/modelmigration"
	"codenest.dev/models/db"
	"codenest.dev/modules/globallock"
)

func Migrate(ctx context.Context, x db.EngineMigration) error {
	// only one instance can do the migration at the same time if there are multiple instances
	release, err := globallock.Lock(ctx, "codenest_versioned_migration")
	if err != nil {
		return err
	}
	defer release()

	return modelmigration.Migrate(ctx, x)
}
