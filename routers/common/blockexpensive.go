// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"strings"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/web/middleware"
	"gitea.dev/modules/web/routing"

	"github.com/go-chi/chi/v5"
)

func BlockExpensive() func(next http.Handler) http.Handler {
	if !setting.Service.BlockAnonymousAccessExpensive {
		return nil
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ret := determineRequestPriority(reqctx.FromContext(req.Context()))
			if !ret.SignedIn {
				if ret.Expensive || ret.LongPolling {
					http.Redirect(w, req, middleware.RedirectLinkUserLogin(req), http.StatusSeeOther)
					return
				}
			}
			next.ServeHTTP(w, req)
		})
	}
}

func isRoutePathExpensive(routePattern string) bool {
	if strings.HasPrefix(routePattern, "/user/") || strings.HasPrefix(routePattern, "/login/") {
		return false
	}

	// an endpoint is expensive if crawlers could consume a lot of resources by accessing it
	expensivePaths := []string{
		// code related
		// media and raw endpoints are for single file accesses and useful for rendering README images, not that expensive
		// the single file endpoints won't be accessed by crawlers as long as the commits/files endpoints are protected from crawlers
		"/{username}/{reponame}/archive/",
		"/{username}/{reponame}/blame/",
		"/{username}/{reponame}/commit/",
		"/{username}/{reponame}/commits/",
		"/{username}/{reponame}/compare/",
		"/{username}/{reponame}/graph",
		"/{username}/{reponame}/rss/branch/",
		"/{username}/{reponame}/src/",

		// issue & PR related (no trailing slash)
		// because it would cause heavy database queries, and PR pages need to load a lot of git data
		"/{username}/{reponame}/issues",
		"/{username}/{reponame}/{type:issues}",
		"/{username}/{reponame}/pulls",
		"/{username}/{reponame}/{type:pulls}",

		// wiki
		"/{username}/{reponame}/wiki/",

		// activity
		"/{username}/{reponame}/activity/",
	}
	for _, path := range expensivePaths {
		if strings.HasPrefix(routePattern, path) {
			return true
		}
	}
	return false
}

func determineRequestPriority(reqCtx reqctx.RequestContext) (ret struct {
	SignedIn    bool
	Expensive   bool
	LongPolling bool
},
) {
	chiRoutePath := chi.RouteContext(reqCtx).RoutePattern()
	if _, ok := reqCtx.GetData()[middleware.ContextDataKeySignedUser].(*user_model.User); ok {
		ret.SignedIn = true
	} else {
		ret.Expensive = isRoutePathExpensive(chiRoutePath)
		ret.LongPolling = routing.GetRequestRecordInfo(reqCtx).IsLongPolling
	}
	return ret
}
