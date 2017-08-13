// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"path"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/cron"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/mailer"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	macaron "gopkg.in/macaron.v1"
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
}

// GlobalInit is for global configuration reload-able.
func GlobalInit() {
	setting.NewContext()
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	models.LoadConfigs()
	NewServices()

	if setting.InstallLock {
		highlight.NewContext()
		markdown.InitMarkdown()
		markdown.NewSanitizer()
		if err := models.NewEngine(migrations.Migrate); err != nil {
			log.Fatal(4, "Failed to initialize ORM engine: %v", err)
		}
		models.HasEngine = true
		models.InitOAuth2()

		models.LoadRepoConfig()
		models.NewRepoContext()

		// Booting long running goroutines.
		cron.NewContext()
		indexer.NewContext()
		models.InitSyncMirrors()
		models.InitDeliverHooks()
		models.InitTestPullRequests()
		log.NewGitLogger(path.Join(setting.LogRootPath, "http.log"))
	}
	if models.EnableSQLite3 {
		log.Info("SQLite3 Supported")
	}
	if models.EnableTiDB {
		log.Info("TiDB Supported")
	}
	checkRunMode()

	if setting.InstallLock && setting.SSH.StartBuiltinServer {
		ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort)
		log.Info("SSH server started on %s:%v", setting.SSH.ListenHost, setting.SSH.ListenPort)
	}
}
