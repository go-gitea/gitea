// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/routing"

	"gitea.com/go-chi/session"
	"github.com/chi-middleware/proxy"
	chi "github.com/go-chi/chi/v5"
)

// ProtocolMiddlewares returns HTTP protocol related middlewares
func ProtocolMiddlewares() (handlers []any) {
	handlers = append(handlers, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// First of all escape the URL RawPath to ensure that all routing is done using a correctly escaped URL
			req.URL.RawPath = req.URL.EscapedPath()

			ctx, _, finished := process.GetManager().AddTypedContext(req.Context(), fmt.Sprintf("%s: %s", req.Method, req.RequestURI), process.RequestProcessType, true)
			defer finished()
			next.ServeHTTP(context.NewResponse(resp), req.WithContext(cache.WithCacheContext(ctx)))
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

	// Strip slashes.
	handlers = append(handlers, stripSlashesMiddleware)

	if !setting.Log.DisableRouterLog {
		handlers = append(handlers, routing.NewLoggerHandler())
	}

	if setting.Log.EnableAccessLog {
		handlers = append(handlers, context.AccessLogger())
	}

	handlers = append(handlers, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// Why we need this? The Recovery() will try to render a beautiful
			// error page for user, but the process can still panic again, and other
			// middleware like session also may panic then we have to recover twice
			// and send a simple error page that should not panic anymore.
			defer func() {
				if err := recover(); err != nil {
					routing.UpdatePanicError(req.Context(), err)
					combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, log.Stack(2))
					log.Error("%v", combinedErr)
					if setting.IsProd {
						http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					} else {
						http.Error(resp, combinedErr, http.StatusInternalServerError)
					}
				}
			}()
			next.ServeHTTP(resp, req)
		})
	})
	return handlers
}

func stripSlashesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		var urlPath string
		rctx := chi.RouteContext(req.Context())
		if rctx != nil && rctx.RoutePath != "" {
			urlPath = rctx.RoutePath
		} else if req.URL.RawPath != "" {
			urlPath = req.URL.RawPath
		} else {
			urlPath = req.URL.Path
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
