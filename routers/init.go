// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/services"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/translation"

	// run init on these packages, don't remove these references except you know what it means
	_ "code.gitea.io/gitea/modules/auth/sso"
	_ "code.gitea.io/gitea/modules/cron"
	_ "code.gitea.io/gitea/modules/eventsource"
	_ "code.gitea.io/gitea/modules/highlight"
	_ "code.gitea.io/gitea/modules/indexer/code"
	_ "code.gitea.io/gitea/modules/indexer/issues"
	_ "code.gitea.io/gitea/modules/indexer/stats"
	_ "code.gitea.io/gitea/modules/markup"
	_ "code.gitea.io/gitea/modules/markup/external"
	_ "code.gitea.io/gitea/modules/migrations"
	_ "code.gitea.io/gitea/modules/notification/action"
	_ "code.gitea.io/gitea/modules/notification/indexer"
	_ "code.gitea.io/gitea/modules/notification/mail"
	_ "code.gitea.io/gitea/modules/notification/ui"
	_ "code.gitea.io/gitea/modules/notification/webhook"
	_ "code.gitea.io/gitea/modules/ssh"
	_ "code.gitea.io/gitea/modules/svg"
	_ "code.gitea.io/gitea/modules/task"
	_ "code.gitea.io/gitea/services/mirror"
	_ "code.gitea.io/gitea/services/pull"
	_ "code.gitea.io/gitea/services/webhook"
)

func checkRunMode() {
	switch setting.RunMode {
	case "dev", "test":
		git.Debug = true
	default:
		git.Debug = false
	}
	log.Info("Run Mode: %s", strings.Title(setting.RunMode))
}

// InitServices init services
func InitServices() {
	if err := services.Init(); err != nil {
		log.Fatal("init services failed: %v", err)
	}
}

// In case of problems connecting to DB, retry connection. Eg, PGSQL in Docker Container on Synology
func initDBEngine(ctx context.Context) (err error) {
	log.Info("Beginning ORM engine initialization.")
	for i := 0; i < setting.Database.DBConnectRetries; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted due to shutdown:\nin retry ORM engine initialization")
		default:
		}
		log.Info("ORM engine initialization attempt #%d/%d...", i+1, setting.Database.DBConnectRetries)
		if err = models.NewEngine(ctx, migrations.Migrate); err == nil {
			break
		} else if i == setting.Database.DBConnectRetries-1 {
			return err
		}
		log.Error("ORM engine initialization attempt #%d/%d failed. Error: %v", i+1, setting.Database.DBConnectRetries, err)
		log.Info("Backing off for %d seconds", int64(setting.Database.DBConnectBackoff/time.Second))
		time.Sleep(setting.Database.DBConnectBackoff)
	}
	models.HasEngine = true
	return nil
}

// PreInstallInit preloads the configuration to check if we need to run install
func PreInstallInit(ctx context.Context) bool {
	setting.NewContext()
	if !setting.InstallLock {
		log.Trace("AppPath: %s", setting.AppPath)
		log.Trace("AppWorkPath: %s", setting.AppWorkPath)
		log.Trace("Custom path: %s", setting.CustomPath)
		log.Trace("Log path: %s", setting.LogRootPath)
		log.Trace("Preparing to run install page")
		translation.InitLocales()
		if setting.EnableSQLite3 {
			log.Info("SQLite3 Supported")
		}
		setting.InitDBConfig()
		svg.Init()
	}

	return !setting.InstallLock
}

// PostInstallInit rereads the settings and starts up the database
func PostInstallInit(ctx context.Context) {
	setting.NewContext()
	setting.InitDBConfig()
	if setting.InstallLock {
		if err := initDBEngine(ctx); err == nil {
			log.Info("ORM engine initialization successful!")
		} else {
			log.Fatal("ORM engine initialization failed: %v", err)
		}
		svg.Init()
	}
}

// GlobalInit is for global configuration reload-able.
func GlobalInit(ctx context.Context) {
	setting.NewContext()
	if !setting.InstallLock {
		log.Fatal("Gitea is not installed")
	}

	if err := git.Init(ctx); err != nil {
		log.Fatal("Git module init failed: %v", err)
	}
	setting.CheckLFSVersion()
	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	checkRunMode()

	// Setup i18n
	translation.InitLocales()

	if setting.EnableSQLite3 {
		log.Info("SQLite3 Supported")
	} else if setting.Database.UseSQLite3 {
		log.Fatal("SQLite3 is set in settings but NOT Supported")
	}
	if err := initDBEngine(ctx); err == nil {
		log.Info("ORM engine initialization successful!")
	} else {
		log.Fatal("ORM engine initialization failed: %v", err)
	}

	// Init all services
	InitServices()

	if err := models.InitOAuth2(); err != nil {
		log.Fatal("Failed to initialize OAuth2 support: %v", err)
	}
}
