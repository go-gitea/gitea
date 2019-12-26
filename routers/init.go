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
	"code.gitea.io/gitea/modules/auth/sso"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/cron"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/external"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/mailer"
	mirror_service "code.gitea.io/gitea/services/mirror"
	pull_service "code.gitea.io/gitea/services/pull"

	"gitea.com/macaron/macaron"
)

func checkRunMode() {
	switch setting.Cfg.Section("").Key("RUN_MODE").String() {
	case "prod":
		macaron.Env = macaron.PROD
		macaron.ColorLog = false
		setting.ProdMode = true
	default:
		git.Debug = true
	}
	log.Info("Run Mode: %s", strings.Title(macaron.Env))
}

// NewServices init new services
func NewServices() {
	setting.NewServices()
	mailer.NewContext()
	_ = cache.NewContext()
	notification.NewContext()
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

// GlobalInit is for global configuration reload-able.
func GlobalInit(ctx context.Context) {
	setting.NewContext()
	if err := git.Init(ctx); err != nil {
		log.Fatal("Git module init failed: %v", err)
	}
	setting.CheckLFSVersion()
	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)

	NewServices()

	if setting.InstallLock {
		highlight.NewContext()
		external.RegisterParsers()
		markup.Init()
		if err := initDBEngine(ctx); err == nil {
			log.Info("ORM engine initialization successful!")
		} else {
			log.Fatal("ORM engine initialization failed: %v", err)
		}

		if err := models.InitOAuth2(); err != nil {
			log.Fatal("Failed to initialize OAuth2 support: %v", err)
		}

		models.NewRepoContext()

		// Booting long running goroutines.
		cron.NewContext()
		issue_indexer.InitIssueIndexer(false)
		code_indexer.Init()
		mirror_service.InitSyncMirrors()
		webhook.InitDeliverHooks()
		pull_service.Init()
		if err := task.Init(); err != nil {
			log.Fatal("Failed to initialize task scheduler: %v", err)
		}
	}
	if setting.EnableSQLite3 {
		log.Info("SQLite3 Supported")
	}
	checkRunMode()

	// Now because Install will re-run GlobalInit once it has set InstallLock
	// we can't tell if the ssh port will remain unused until that's done.
	// However, see FIXME comment in install.go
	if setting.InstallLock {
		if setting.SSH.StartBuiltinServer {
			ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
			log.Info("SSH server started on %s:%d. Cipher list (%v), key exchange algorithms (%v), MACs (%v)", setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
		} else {
			ssh.Unused()
		}
	}

	if setting.InstallLock {
		sso.Init()
	}
}
