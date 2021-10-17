// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"context"
	"reflect"
	"runtime"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/appstate"
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
	repo_migrations "code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web"
	apiv1 "code.gitea.io/gitea/routers/api/v1"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/routers/private"
	web_routers "code.gitea.io/gitea/routers/web"
	"code.gitea.io/gitea/services/archiver"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/mailer"
	mirror_service "code.gitea.io/gitea/services/mirror"
	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/services/webhook"

	"gitea.com/go-chi/session"
)

func guaranteeInit(fn func() error) {
	err := fn()
	if err != nil {
		ptr := reflect.ValueOf(fn).Pointer()
		fi := runtime.FuncForPC(ptr)
		log.Fatal("%s init failed: %v", fi.Name(), err)
	}
}

func guaranteeInitCtx(ctx context.Context, fn func(ctx context.Context) error) {
	err := fn(ctx)
	if err != nil {
		ptr := reflect.ValueOf(fn).Pointer()
		fi := runtime.FuncForPC(ptr)
		log.Fatal("%s(ctx) init failed: %v", fi.Name(), err)
	}
}

// InitGitServices init new services for git, this is also called in `contrib/pr/checkout.go`
func InitGitServices() {
	setting.NewServices()
	guaranteeInit(storage.Init)
	guaranteeInit(repository.NewContext)
}

func syncAppPathForGitHooks(ctx context.Context) (err error) {
	runtimeState := new(appstate.RuntimeState)
	if err = setting.AppState.Get(runtimeState); err != nil {
		return err
	}
	if runtimeState.LastAppPath != setting.AppPath {
		log.Info("AppPath changed from '%s' to '%s', sync repository hooks ...", runtimeState.LastAppPath, setting.AppPath)
		err = repo_module.SyncRepositoryHooks(ctx)
		if err != nil {
			return err
		}
		runtimeState.LastAppPath = setting.AppPath
		return setting.AppState.Set(runtimeState)
	}
	return nil
}

// GlobalInit is for global configuration reload-able.
func GlobalInit(ctx context.Context) {
	setting.NewContext()
	if !setting.InstallLock {
		log.Fatal("Gitea is not installed")
	}

	guaranteeInitCtx(ctx, git.Init)
	log.Info(git.VersionInfo())

	git.CheckLFSVersion()
	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.LogRootPath)
	log.Info("Configuration file: %s", setting.CustomConf)
	log.Info("Run Mode: %s", strings.Title(setting.RunMode))

	// Setup i18n
	translation.InitLocales()

	InitGitServices()
	mailer.NewContext()
	guaranteeInit(cache.NewContext)
	notification.NewContext()
	guaranteeInit(archiver.Init)

	highlight.NewContext()
	external.RegisterRenderers()
	markup.Init()

	if setting.EnableSQLite3 {
		log.Info("SQLite3 Supported")
	} else if setting.Database.UseSQLite3 {
		log.Fatal("SQLite3 is set in settings but NOT Supported")
	}

	guaranteeInitCtx(ctx, common.InitDBEngine)
	log.Info("ORM engine initialization successful!")

	guaranteeInit(oauth2.Init)

	models.NewRepoContext()

	// Booting long running goroutines.
	cron.NewContext()
	issue_indexer.InitIssueIndexer(false)
	code_indexer.Init()
	guaranteeInit(stats_indexer.Init)

	mirror_service.InitSyncMirrors()
	webhook.InitDeliverHooks()
	guaranteeInit(pull_service.Init)
	guaranteeInit(task.Init)
	guaranteeInit(repo_migrations.Init)
	eventsource.GetManager().Init()

	guaranteeInitCtx(ctx, syncAppPathForGitHooks)

	if setting.SSH.StartBuiltinServer {
		ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
		log.Info("SSH server started on %s:%d. Cipher list (%v), key exchange algorithms (%v), MACs (%v)", setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
	} else {
		ssh.Unused()
	}
	auth.Init()
	svg.Init()
}

// NormalRoutes represents non install routes
func NormalRoutes() *web.Route {
	r := web.NewRoute()
	for _, middle := range common.Middlewares() {
		r.Use(middle)
	}

	sessioner := session.Sessioner(session.Options{
		Provider:       setting.SessionConfig.Provider,
		ProviderConfig: setting.SessionConfig.ProviderConfig,
		CookieName:     setting.SessionConfig.CookieName,
		CookiePath:     setting.SessionConfig.CookiePath,
		Gclifetime:     setting.SessionConfig.Gclifetime,
		Maxlifetime:    setting.SessionConfig.Maxlifetime,
		Secure:         setting.SessionConfig.Secure,
		SameSite:       setting.SessionConfig.SameSite,
		Domain:         setting.SessionConfig.Domain,
	})

	r.Mount("/", web_routers.Routes(sessioner))
	r.Mount("/api/v1", apiv1.Routes(sessioner))
	r.Mount("/api/internal", private.Routes())
	return r
}
