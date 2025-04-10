// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"

	"github.com/go-chi/chi/v5"
	lru "github.com/hashicorp/golang-lru/v2"
)

const tplStatus503RateLimit templates.TplName = "status/503_ratelimit"

type RateLimitToken struct {
	RetryAfter time.Time
}

func BlockExpensive() func(next http.Handler) http.Handler {
	if !setting.Service.BlockAnonymousAccessExpensive && !setting.Service.BlockAnonymousAccessOverload {
		return nil
	}

	tokenCache, _ := lru.New[string, RateLimitToken](10000)

	deferAnonymousRateLimitAccess := func(w http.ResponseWriter, req *http.Request) bool {
		// * For a crawler: if it sees 503 error, it would retry later (they have their own queue), there is still a chance for them to read all pages
		// * For a real anonymous user: allocate a token, and let them wait for a while by browser JS (queue the request by browser)

		const tokenCookieName = "gitea_arlt" // gitea anonymous rate limit token
		cookieToken, _ := req.Cookie(tokenCookieName)
		if cookieToken != nil && cookieToken.Value != "" {
			token, exist := tokenCache.Get(cookieToken.Value)
			if exist {
				if time.Now().After(token.RetryAfter) {
					// still valid
					tokenCache.Remove(cookieToken.Value)
					return false
				}
				// not reach RetryAfter time, so either remove the old one and allocate a new one, or keep using the old one
				// TODO: in the future, we could do better to allow more accesses for the same token, or extend the expiration time if the access seems well-behaved
				tokenCache.Remove(cookieToken.Value)
			}
		}

		// TODO: merge the code with RenderPanicErrorPage
		tmplCtx := context.TemplateContext{}
		tmplCtx["Locale"] = middleware.Locale(w, req)
		ctxData := middleware.GetContextData(req.Context())

		tokenKey, _ := util.CryptoRandomString(32)
		retryAfterDuration := 1 * time.Second
		token := RateLimitToken{RetryAfter: time.Now().Add(retryAfterDuration)}
		tokenCache.Add(tokenKey, token)
		ctxData["RateLimitTokenKey"] = tokenKey
		ctxData["RateLimitCookieName"] = tokenCookieName
		ctxData["RateLimitRetryAfterMs"] = retryAfterDuration.Milliseconds() + 100
		_ = templates.HTMLRenderer().HTML(w, http.StatusServiceUnavailable, tplStatus503RateLimit, ctxData, tmplCtx)
		return true
	}

	inflightRequestNum := atomic.Int32{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ret := determineRequestPriority(reqctx.FromContext(req.Context()))
			if !ret.SignedIn {
				if ret.LongPolling {
					http.Error(w, "Long polling is not allowed for anonymous users", http.StatusForbidden)
					return
				}
				if ret.Expensive {
					inflightNum := inflightRequestNum.Add(1)
					defer inflightRequestNum.Add(-1)

					if setting.Service.BlockAnonymousAccessExpensive {
						// strictly block the anonymous accesses to expensive pages, to save CPU
						http.Redirect(w, req, setting.AppSubURL+"/user/login", http.StatusSeeOther)
						return
					} else if int(inflightNum) > setting.Service.OverloadInflightAnonymousRequests {
						// be friendly to anonymous access (crawler, real anonymous user) to expensive pages, but limit the inflight requests
						if deferAnonymousRateLimitAccess(w, req) {
							return
						}
					}
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

	expensivePaths := []string{
		// code related
		"/{username}/{reponame}/archive/",
		"/{username}/{reponame}/blame/",
		"/{username}/{reponame}/commit/",
		"/{username}/{reponame}/commits/",
		"/{username}/{reponame}/compare/",
		"/{username}/{reponame}/graph",
		"/{username}/{reponame}/media/",
		"/{username}/{reponame}/raw/",
		"/{username}/{reponame}/src/",

		// issue & PR related (no trailing slash)
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

func isRoutePathForLongPolling(routePattern string) bool {
	return routePattern == "/user/events"
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
		ret.LongPolling = isRoutePathForLongPolling(chiRoutePath)
	}
	return ret
}
