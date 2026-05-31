// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	"fmt"
	"html"
	"net/http"

	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/web"
	"gitea.dev/routers/common"
	"gitea.dev/routers/web/healthcheck"
	"gitea.dev/routers/web/misc"
	"gitea.dev/services/forms"
)

// Routes registers the installation routes
func Routes() *web.Router {
	base := web.NewRouter()
	base.BeforeRouting(common.ProtocolMiddlewares()...)

	base.Methods("GET, HEAD", "/assets/*", public.FileHandlerFunc())

	r := web.NewRouter()
	r.AfterRouting(common.MustInitSessioner(), installContexter())

	r.Get("/", Install) // it must be on the root, because the "install.js" use the window.location to replace the "localhost" AppURL
	r.Post("/", web.Bind(forms.InstallForm{}), SubmitInstall)
	r.Get("/post-install", InstallDone)

	r.Get("/-/web-theme/list", misc.WebThemeList)
	r.Post("/-/web-theme/apply", misc.WebThemeApply)
	r.Get("/api/healthz", healthcheck.Check)

	r.NotFound(installNotFound)

	base.Mount("", r)
	return base
}

func installNotFound(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Header().Add("Refresh", "1; url="+setting.AppSubURL+"/")
	// do not use 30x status, because the "post-install" page needs to use 404/200 to detect if Gitea has been installed.
	// the fetch API could follow 30x requests to the page with 200 status.
	w.WriteHeader(http.StatusNotFound)
	_, _ = fmt.Fprintf(w, `Not Found. <a href="%s">Go to default page</a>.`, html.EscapeString(setting.AppSubURL+"/"))
}
