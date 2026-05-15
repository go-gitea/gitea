// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/log"
)

func checkDBVersion(ctx context.Context, logger log.Logger, autofix bool) error {
	logger.Info("Expected database version: %d", migrations.ExpectedDBVersion())
	if err := db.InitEngineWithValidation(ctx, migrations.EnsureUpToDate); err != nil {
		markDatabaseUntrusted(ctx)
		logger.Critical("Error: %v during ensure up to date", err)
		if autofix {
			logger.Warn("Automatic database migration is disabled for doctor; upgrade the database separately before running destructive fixes")
		}
		return err
	}
	return nil
}

func init() {
	Register(&Check{
		Title:         "Check Database Version",
		Name:          "check-db-version",
		IsDefault:     true,
		Run:           checkDBVersion,
		AbortIfFailed: false,
		Priority:      2,
	})
}
