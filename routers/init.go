// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routers

import (
	"context"
	"net/http"
	"reflect"
	"runtime"

	"gitea.dev/models"
	authmodel "gitea.dev/models/auth"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/eventsource"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/external"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/ssh"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/svg"
	"gitea.dev/modules/system"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/modules/web/routing"
	actions_router "gitea.dev/routers/api/actions"
	packages_router "gitea.dev/routers/api/packages"
	apiv1 "gitea.dev/routers/api/v1"
	"gitea.dev/routers/common"
	"gitea.dev/routers/private"
	web_routers "gitea.dev/routers/web"
	actions_service "gitea.dev/services/actions"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/auth"
	"gitea.dev/services/auth/source/oauth2"
	"gitea.dev/services/automerge"
	"gitea.dev/services/cron"
	feed_service "gitea.dev/services/feed"
	indexer_service "gitea.dev/services/indexer"
	"gitea.dev/services/mailer"
	mailer_incoming "gitea.dev/services/mailer/incoming"
	markup_service "gitea.dev/services/markup"
	repo_migrations "gitea.dev/services/migrations"
	mirror_service "gitea.dev/services/mirror"
	"gitea.dev/services/oauth2_provider"
	packages_spec "gitea.dev/services/packages/pkgspec"
	pull_service "gitea.dev/services/pull"
	release_service "gitea.dev/services/release"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/services/repository/archiver"
	"gitea.dev/services/task"
	"gitea.dev/services/uinotification"
	"gitea.dev/services/webhook"
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
	if err := system.AppState.Get(ctx, runtimeState); err != nil {
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
		mustInitCtx(ctx, asymkey_service.RewriteAllPublicKeys)

		return system.AppState.Set(ctx, runtimeState)
	}
	return nil
}

func InitWebInstallPage(ctx context.Context) {
	translation.InitLocales(ctx)
	setting.LoadSettingsForInstall()
	mustInit(svg.Init)
}

// InitWebInstalled is for the global configuration of an installed instance
func InitWebInstalled(ctx context.Context) {
	mustInit(git.InitFull)
	log.Info("Git version: %s (home: %s)", git.DefaultFeatures().VersionInfo(), gitcmd.HomeDir())
	if !git.DefaultFeatures().SupportHashSha256 {
		log.Warn("sha256 hash support is disabled - requires Git >= 2.42." + util.Iif(git.DefaultFeatures().UsingGogit, " Gogit is currently unsupported.", ""))
	}

	// Setup i18n
	translation.InitLocales(ctx)

	setting.LoadSettings()
	mustInit(storage.Init)

	mailer.NewContext(ctx)
	mustInit(cache.Init)
	mustInit(feed_service.Init)
	mustInit(uinotification.Init)
	mustInitCtx(ctx, archiver.Init)

	external.RegisterRenderers()
	markup.Init(markup_service.FormalRenderHelperFuncs())

	mustInitCtx(ctx, common.InitDBEngine)
	log.Info("ORM engine initialization successful!")
	mustInit(system.Init)
	mustInitCtx(ctx, oauth2.Init)
	mustInitCtx(ctx, oauth2_provider.Init)
	mustInit(release_service.Init)

	mustInitCtx(ctx, models.Init)
	mustInitCtx(ctx, authmodel.Init)
	mustInitCtx(ctx, repo_service.Init)
	mustInit(packages_spec.InitManager)

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

	mustInitCtx(ctx, actions_service.Init)

	mustInit(repo_service.InitLicenseClassifier)

	// Finally start up the cron
	cron.Init(ctx)
}

// NormalRoutes represents non install routes
func NormalRoutes() *web.Router {
	r := web.NewRouter()
	r.BeforeRouting(common.ProtocolMiddlewares()...)

	r.AfterRouting(common.MaintenanceModeHandler())

	r.Mount("/", web_routers.Routes())
	r.Mount("/api/v1", apiv1.Routes())
	r.Mount("/api/internal", private.Routes())

	r.Post("/-/fetch-redirect", common.FetchRedirectDelegate)

	if setting.Packages.Enabled {
		// This implements package support for most package managers
		r.Mount("/api/packages", packages_router.CommonRoutes())
		// This implements the OCI API, this container registry "/v2" endpoint must be in the root of the site.
		// If site admin deploys Gitea in a sub-path, they must configure their reverse proxy to map the "https://host/v2" endpoint to Gitea.
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
		prefix = actions_router.ArtifactV4RouteBase
		r.Mount(prefix, actions_router.ArtifactsV4Routes(prefix))
	}

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		defer routing.RecordFuncInfo(req.Context(), routing.GetFuncInfo(http.NotFound, "GlobalNotFound"))()
		http.NotFound(w, req)
	})
	return r
}
