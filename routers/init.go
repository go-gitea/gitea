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
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	stats_indexer "code.gitea.io/gitea/modules/indexer/stats"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/external"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/mailer"
	mirror_service "code.gitea.io/gitea/services/mirror"
	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/services/repository"

	"gitea.com/macaron/i18n"
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
	if err := storage.Init(); err != nil {
		log.Fatal("storage init failed: %v", err)
	}
	if err := repository.NewContext(); err != nil {
		log.Fatal("repository init failed: %v", err)
	}
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

// InitLocales loads the locales
func InitLocales() {
	localeNames, err := options.Dir("locale")

	if err != nil {
		log.Fatal("Failed to list locale files: %v", err)
	}
	localFiles := make(map[string][]byte)

	for _, name := range localeNames {
		localFiles[name], err = options.Locale(name)

		if err != nil {
			log.Fatal("Failed to load %s locale file. %v", name, err)
		}
	}
	i18n.I18n(i18n.Options{
		SubURL:       setting.AppSubURL,
		Files:        localFiles,
		Langs:        setting.Langs,
		Names:        setting.Names,
		DefaultLang:  "en-US",
		Redirect:     false,
		CookieDomain: setting.SessionConfig.Domain,
	})
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
		InitLocales()
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

	// Setup i18n
	InitLocales()

	NewServices()

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
	if err := stats_indexer.Init(); err != nil {
		log.Fatal("Failed to initialize repository stats indexer queue: %v", err)
	}
	mirror_service.InitSyncMirrors()
	webhook.InitDeliverHooks()
	if err := pull_service.Init(); err != nil {
		log.Fatal("Failed to initialize test pull requests queue: %v", err)
	}
	if err := task.Init(); err != nil {
		log.Fatal("Failed to initialize task scheduler: %v", err)
	}
	eventsource.GetManager().Init()

	if setting.EnableSQLite3 {
		log.Info("SQLite3 Supported")
	}
	checkRunMode()

	if setting.SSH.StartBuiltinServer {
		ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
		log.Info("SSH server started on %s:%d. Cipher list (%v), key exchange algorithms (%v), MACs (%v)", setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
	} else {
		ssh.Unused()
	}
	sso.Init()

	svg.Init()
}
