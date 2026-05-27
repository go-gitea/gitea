// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"errors"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/migrations"
	system_model "gitea.dev/models/system"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/setting/config"
	"gitea.dev/services/versioned_migration"
)

// InitDBEngine In case of problems connecting to DB, retry connection. Eg, PGSQL in Docker Container on Synology
func InitDBEngine(ctx context.Context) (err error) {
	log.Info("Beginning ORM engine initialization.")
	for i := 0; i < setting.Database.DBConnectRetries; i++ {
		select {
		case <-ctx.Done():
			return errors.New("Aborted due to shutdown:\nin retry ORM engine initialization")
		default:
		}
		log.Info("ORM engine initialization attempt #%d/%d...", i+1, setting.Database.DBConnectRetries)
		if err = db.InitEngineWithMigration(ctx, migrateWithSetting); err == nil {
			break
		} else if i == setting.Database.DBConnectRetries-1 {
			return err
		}
		log.Error("ORM engine initialization attempt #%d/%d failed. Error: %v", i+1, setting.Database.DBConnectRetries, err)
		log.Info("Backing off for %d seconds", int64(setting.Database.DBConnectBackoff/time.Second))
		time.Sleep(setting.Database.DBConnectBackoff)
	}
	config.SetDynGetter(system_model.NewDatabaseDynKeyGetter())
	return nil
}

func migrateWithSetting(ctx context.Context, x db.EngineMigration) error {
	if setting.Database.AutoMigration {
		return versioned_migration.Migrate(ctx, x)
	}

	if current, err := migrations.GetCurrentDBVersion(x); err != nil {
		return err
	} else if current < 0 {
		// execute migrations when the database isn't initialized even if AutoMigration is false
		return versioned_migration.Migrate(ctx, x)
	} else if expected := migrations.ExpectedDBVersion(); current != expected {
		log.Fatal(`"database.AUTO_MIGRATION" is disabled, but current database version %d is not equal to the expected version %d.`+
			`You can set "database.AUTO_MIGRATION" to true or migrate manually by running "gitea [--config /path/to/app.ini] migrate"`, current, expected)
	}
	return nil
}
