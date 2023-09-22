// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routers

import (
	"context"
	"reflect"
	"runtime"

	"code.gitea.io/gitea/internal/models"
	asymkey_model "code.gitea.io/gitea/internal/models/asymkey"
	authmodel "code.gitea.io/gitea/internal/models/auth"
	"code.gitea.io/gitea/internal/modules/cache"
	"code.gitea.io/gitea/internal/modules/eventsource"
	"code.gitea.io/gitea/internal/modules/git"
	"code.gitea.io/gitea/internal/modules/highlight"
	"code.gitea.io/gitea/internal/modules/log"
	"code.gitea.io/gitea/internal/modules/markup"
	"code.gitea.io/gitea/internal/modules/markup/external"
	"code.gitea.io/gitea/internal/modules/setting"
	"code.gitea.io/gitea/internal/modules/ssh"
	"code.gitea.io/gitea/internal/modules/storage"
	"code.gitea.io/gitea/internal/modules/svg"
	"code.gitea.io/gitea/internal/modules/system"
	"code.gitea.io/gitea/internal/modules/templates"
	"code.gitea.io/gitea/internal/modules/translation"
	"code.gitea.io/gitea/internal/modules/web"
	actions_router "code.gitea.io/gitea/internal/routers/api/actions"
	packages_router "code.gitea.io/gitea/internal/routers/api/packages"
	apiv1 "code.gitea.io/gitea/internal/routers/api/v1"
	"code.gitea.io/gitea/internal/routers/common"
	"code.gitea.io/gitea/internal/routers/private"
	web_routers "code.gitea.io/gitea/internal/routers/web"
	actions_service "code.gitea.io/gitea/internal/services/actions"
	"code.gitea.io/gitea/internal/services/auth"
	"code.gitea.io/gitea/internal/services/auth/source/oauth2"
	"code.gitea.io/gitea/internal/services/automerge"
	"code.gitea.io/gitea/internal/services/cron"
	feed_service "code.gitea.io/gitea/internal/services/feed"
	indexer_service "code.gitea.io/gitea/internal/services/indexer"
	"code.gitea.io/gitea/internal/services/mailer"
	mailer_incoming "code.gitea.io/gitea/internal/services/mailer/incoming"
	markup_service "code.gitea.io/gitea/internal/services/markup"
	repo_migrations "code.gitea.io/gitea/internal/services/migrations"
	mirror_service "code.gitea.io/gitea/internal/services/mirror"
	pull_service "code.gitea.io/gitea/internal/services/pull"
	repo_service "code.gitea.io/gitea/internal/services/repository"
	"code.gitea.io/gitea/internal/services/repository/archiver"
	"code.gitea.io/gitea/internal/services/task"
	"code.gitea.io/gitea/internal/services/uinotification"
	"code.gitea.io/gitea/internal/services/webhook"
)

func mustInit(fn func() error) {
	err := fn()
	if err != nil {
		ptr := reflect.ValueOf(fn).Pointer()
		fi := runtime.FuncForPC(ptr)
		log.Fatal("%s failed: %v", fi.Name(), err)
	}
}

func mustInitCtx(ctx context.Context, fn func(ctx context.Context) error) {
	err := fn(ctx)
	if err != nil {
		ptr := reflect.ValueOf(fn).Pointer()
		fi := runtime.FuncForPC(ptr)
		log.Fatal("%s(ctx) failed: %v", fi.Name(), err)
	}
}

func syncAppConfForGit(ctx context.Context) error {
	runtimeState := new(system.RuntimeState)
	if err := system.AppState.Get(runtimeState); err != nil {
		return err
	}

	updated := false
	if runtimeState.LastAppPath != setting.AppPath {
		log.Info("AppPath changed from '%s' to '%s'", runtimeState.LastAppPath, setting.AppPath)
		runtimeState.LastAppPath = setting.AppPath
		updated = true
	}
	if runtimeState.LastCustomConf != setting.CustomConf {
		log.Info("CustomConf changed from '%s' to '%s'", runtimeState.LastCustomConf, setting.CustomConf)
		runtimeState.LastCustomConf = setting.CustomConf
		updated = true
	}

	if updated {
		log.Info("re-sync repository hooks ...")
		mustInitCtx(ctx, repo_service.SyncRepositoryHooks)

		log.Info("re-write ssh public keys ...")
		mustInit(asymkey_model.RewriteAllPublicKeys)

		return system.AppState.Set(runtimeState)
	}
	return nil
}

func InitWebInstallPage(ctx context.Context) {
	translation.InitLocales(ctx)
	setting.LoadSettingsForInstall()
	mustInit(svg.Init)
}

// InitWebInstalled is for global installed configuration.
func InitWebInstalled(ctx context.Context) {
	mustInitCtx(ctx, git.InitFull)
	log.Info("Git version: %s (home: %s)", git.VersionInfo(), git.HomeDir())

	// Setup i18n
	translation.InitLocales(ctx)

	setting.LoadSettings()
	mustInit(storage.Init)

	mailer.NewContext(ctx)
	mustInit(cache.NewContext)
	mustInit(feed_service.Init)
	mustInit(uinotification.Init)
	mustInit(archiver.Init)

	highlight.NewContext()
	external.RegisterRenderers()
	markup.Init(markup_service.ProcessorHelper())

	if setting.EnableSQLite3 {
		log.Info("SQLite3 support is enabled")
	} else if setting.Database.Type.IsSQLite3() {
		log.Fatal("SQLite3 support is disabled, but it is used for database setting. Please get or build a Gitea release with SQLite3 support.")
	}

	mustInitCtx(ctx, common.InitDBEngine)
	log.Info("ORM engine initialization successful!")
	mustInit(system.Init)
	mustInit(oauth2.Init)

	mustInitCtx(ctx, models.Init)
	mustInitCtx(ctx, authmodel.Init)
	mustInitCtx(ctx, repo_service.Init)

	// Booting long running goroutines.
	mustInit(indexer_service.Init)

	mirror_service.InitSyncMirrors()
	mustInit(webhook.Init)
	mustInit(pull_service.Init)
	mustInit(automerge.Init)
	mustInit(task.Init)
	mustInit(repo_migrations.Init)
	eventsource.GetManager().Init()
	mustInitCtx(ctx, mailer_incoming.Init)

	mustInitCtx(ctx, syncAppConfForGit)

	mustInit(ssh.Init)

	auth.Init()
	mustInit(svg.Init)

	actions_service.Init()

	// Finally start up the cron
	cron.NewContext(ctx)
}

// NormalRoutes represents non install routes
func NormalRoutes() *web.Route {
	_ = templates.HTMLRenderer()
	r := web.NewRoute()
	r.Use(common.ProtocolMiddlewares()...)

	r.Mount("/", web_routers.Routes())
	r.Mount("/api/v1", apiv1.Routes())
	r.Mount("/api/internal", private.Routes())

	r.Post("/-/fetch-redirect", common.FetchRedirectDelegate)

	if setting.Packages.Enabled {
		// This implements package support for most package managers
		r.Mount("/api/packages", packages_router.CommonRoutes())
		// This implements the OCI API (Note this is not preceded by /api but is instead /v2)
		r.Mount("/v2", packages_router.ContainerRoutes())
	}

	if setting.Actions.Enabled {
		prefix := "/api/actions"
		r.Mount(prefix, actions_router.Routes(prefix))

		// TODO: Pipeline api used for runner internal communication with gitea server. but only artifact is used for now.
		// In Github, it uses ACTIONS_RUNTIME_URL=https://pipelines.actions.githubusercontent.com/fLgcSHkPGySXeIFrg8W8OBSfeg3b5Fls1A1CwX566g8PayEGlg/
		// TODO: this prefix should be generated with a token string with runner ?
		prefix = "/api/actions_pipeline"
		r.Mount(prefix, actions_router.ArtifactsRoutes(prefix))
	}

	return r
}
