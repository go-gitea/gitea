// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	go_context "context"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/services/context"

	"gitea.com/go-chi/session"
	"github.com/chi-middleware/proxy"
	chi "github.com/go-chi/chi/v5"
)

// ProtocolMiddlewares returns HTTP protocol related middlewares, and it provides a global panic recovery
func ProtocolMiddlewares() (handlers []any) {
	// first, normalize the URL path
	handlers = append(handlers, normalizeRequestPathMiddleware)

	// prepare the ContextData and panic recovery
	handlers = append(handlers, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					RenderPanicErrorPage(resp, req, err) // it should never panic
				}
			}()
			req = req.WithContext(middleware.WithContextData(req.Context()))
			req = req.WithContext(go_context.WithValue(req.Context(), httplib.RequestContextKey, req))
			next.ServeHTTP(resp, req)
		})
	})

	// wrap the request and response, use the process context and add it to the process manager
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

func normalizeRequestPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		// escape the URL RawPath to ensure that all routing is done using a correctly escaped URL
		req.URL.RawPath = req.URL.EscapedPath()

		urlPath := req.URL.RawPath
		rctx := chi.RouteContext(req.Context())
		if rctx != nil && rctx.RoutePath != "" {
			urlPath = rctx.RoutePath
		}

		normalizedPath := strings.TrimRight(urlPath, "/")
		// the following code block is a slow-path for replacing all repeated slashes "//" to one single "/"
		// if the path doesn't have repeated slashes, then no need to execute it
		if strings.Contains(normalizedPath, "//") {
			buf := &strings.Builder{}
			prevWasSlash := false
			for _, chr := range normalizedPath {
				if chr != '/' || !prevWasSlash {
					buf.WriteRune(chr)
				}
				prevWasSlash = chr == '/'
			}
			normalizedPath = buf.String()
		}

		if setting.UseSubURLPath {
			remainingPath, ok := strings.CutPrefix(normalizedPath, setting.AppSubURL+"/")
			if ok {
				normalizedPath = "/" + remainingPath
			} else if normalizedPath == setting.AppSubURL {
				normalizedPath = "/"
			} else if !strings.HasPrefix(normalizedPath+"/", "/v2/") {
				// do not respond to other requests, to simulate a real sub-path environment
				http.Error(resp, "404 page not found, sub-path is: "+setting.AppSubURL, http.StatusNotFound)
				return
			}
			// TODO: it's not quite clear about how req.URL and rctx.RoutePath work together.
			// Fortunately, it is only used for debug purpose, we have enough time to figure it out in the future.
			req.URL.RawPath = normalizedPath
			req.URL.Path = normalizedPath
		}

		if rctx == nil {
			req.URL.Path = normalizedPath
		} else {
			rctx.RoutePath = normalizedPath
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
