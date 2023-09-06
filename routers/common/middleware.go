// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/modules/web/routing"

	"gitea.com/go-chi/session"
	"github.com/chi-middleware/proxy"
	chi "github.com/go-chi/chi/v5"
)

// ProtocolMiddlewares returns HTTP protocol related middlewares, and it provides a global panic recovery
func ProtocolMiddlewares() (handlers []any) {
	// first, normalize the URL path
	handlers = append(handlers, stripSlashesMiddleware)

	// prepare the ContextData and panic recovery
	handlers = append(handlers, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					RenderPanicErrorPage(resp, req, err) // it should never panic
				}
			}()
			req = req.WithContext(middleware.WithContextData(req.Context()))
			next.ServeHTTP(resp, req)
		})
	})

	handlers = append(handlers, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx, _, finished := process.GetManager().AddTypedContext(req.Context(), fmt.Sprintf("%s: %s", req.Method, req.RequestURI), process.RequestProcessType, true)
			defer finished()
			next.ServeHTTP(context.WrapResponseWriter(resp), req.WithContext(cache.WithCacheContext(ctx)))
		})
	})

	if setting.ReverseProxyLimit > 0 {
		opt := proxy.NewForwardedHeadersOptions().
			WithForwardLimit(setting.ReverseProxyLimit).
			ClearTrustedProxies()
		for _, n := range setting.ReverseProxyTrustedProxies {
			if !strings.Contains(n, "/") {
				opt.AddTrustedProxy(n)
			} else {
				opt.AddTrustedNetwork(n)
			}
		}
		handlers = append(handlers, proxy.ForwardedHeaders(opt))
	}

	if setting.IsRouteLogEnabled() {
		handlers = append(handlers, routing.NewLoggerHandler())
	}

	if setting.IsAccessLogEnabled() {
		handlers = append(handlers, context.AccessLogger())
	}

	return handlers
}

func stripSlashesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		// First of all escape the URL RawPath to ensure that all routing is done using a correctly escaped URL
		req.URL.RawPath = req.URL.EscapedPath()

		urlPath := req.URL.RawPath
		rctx := chi.RouteContext(req.Context())
		if rctx != nil && rctx.RoutePath != "" {
			urlPath = rctx.RoutePath
		}

		sanitizedPath := &strings.Builder{}
		prevWasSlash := false
		for _, chr := range strings.TrimRight(urlPath, "/") {
			if chr != '/' || !prevWasSlash {
				sanitizedPath.WriteRune(chr)
			}
			prevWasSlash = chr == '/'
		}

		if rctx == nil {
			req.URL.Path = sanitizedPath.String()
		} else {
			rctx.RoutePath = sanitizedPath.String()
		}
		next.ServeHTTP(resp, req)
	})
}

func Sessioner() func(next http.Handler) http.Handler {
	return session.Sessioner(session.Options{
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
}
