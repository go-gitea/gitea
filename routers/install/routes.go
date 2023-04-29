// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	goctx "context"
	"fmt"
	"html"
	"net/http"

	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/routers/web/healthcheck"
	"code.gitea.io/gitea/services/forms"
)

type dataStore map[string]interface{}

func (d *dataStore) GetData() map[string]interface{} {
	return *d
}

func installRecovery(ctx goctx.Context) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				// Why we need this? The first recover will try to render a beautiful
				// error page for user, but the process can still panic again, then
				// we have to just recover twice and send a simple error page that
				// should not panic anymore.
				defer func() {
					if err := recover(); err != nil {
						combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, log.Stack(2))
						log.Error("%s", combinedErr)
						if setting.IsProd {
							http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						} else {
							http.Error(w, combinedErr, http.StatusInternalServerError)
						}
					}
				}()

				if err := recover(); err != nil {
					combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, log.Stack(2))
					log.Error("%s", combinedErr)

					lc := middleware.Locale(w, req)
					store := dataStore{
						"Language":       lc.Language(),
						"CurrentURL":     setting.AppSubURL + req.URL.RequestURI(),
						"locale":         lc,
						"SignedUserID":   int64(0),
						"SignedUserName": "",
					}

					httpcache.SetCacheControlInHeader(w.Header(), 0, "no-transform")
					w.Header().Set(`X-Frame-Options`, setting.CORSConfig.XFrameOptions)

					if !setting.IsProd {
						store["ErrorMsg"] = combinedErr
					}
					_, rnd := templates.HTMLRenderer(ctx)
					err = rnd.HTML(w, http.StatusInternalServerError, "status/500", templates.BaseVars().Merge(store))
					if err != nil {
						log.Error("%v", err)
					}
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}

// Routes registers the installation routes
func Routes(ctx goctx.Context) *web.Route {
	base := web.NewRoute()
	base.Use(common.ProtocolMiddlewares()...)
	base.RouteMethods("/assets/*", "GET, HEAD", public.AssetsHandlerFunc("/assets/"))

	r := web.NewRoute()
	r.Use(common.Sessioner())
	r.Use(installRecovery(ctx))
	r.Use(Init(ctx))
	r.Get("/", Install) // it must be on the root, because the "install.js" use the window.location to replace the "localhost" AppURL
	r.Post("/", web.Bind(forms.InstallForm{}), SubmitInstall)
	r.Get("/post-install", InstallDone)
	r.Get("/api/healthz", healthcheck.Check)
	r.NotFound(installNotFound)

	base.Mount("", r)
	return base
}

func installNotFound(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Header().Add("Refresh", fmt.Sprintf("1; url=%s", setting.AppSubURL+"/"))
	// do not use 30x status, because the "post-install" page needs to use 404/200 to detect if Gitea has been installed.
	// the fetch API could follow 30x requests to the page with 200 status.
	w.WriteHeader(http.StatusNotFound)
	_, _ = fmt.Fprintf(w, `Not Found. <a href="%s">Go to default page</a>.`, html.EscapeString(setting.AppSubURL+"/"))
}
