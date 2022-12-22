// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/routing"

	"github.com/chi-middleware/proxy"
	"github.com/go-chi/chi/v5/middleware"
)

// Middlewares returns common middlewares
func Middlewares() []func(http.Handler) http.Handler {
	handlers := []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				// First of all escape the URL RawPath to ensure that all routing is done using a correctly escaped URL
				req.URL.RawPath = req.URL.EscapedPath()

				ctx, _, finished := process.GetManager().AddTypedContext(req.Context(), fmt.Sprintf("%s: %s", req.Method, req.RequestURI), process.RequestProcessType, true)
				defer finished()
				next.ServeHTTP(context.NewResponse(resp), req.WithContext(ctx))
			})
		},
	}

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

	handlers = append(handlers, middleware.StripSlashes)

	if !setting.DisableRouterLog {
		handlers = append(handlers, routing.NewLoggerHandler())
	}

	if setting.EnableAccessLog {
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

	// Add CSRF handler.
	handlers = append(handlers, csrfHandler())

	return handlers
}

// csfrHandler blocks recognized CSRF attempts.
// WARNING: for this proctection to work, web browser compatible with
// Fetch Metadata Request Headers (https://w3c.github.io/webappsec-fetch-metadata)
// must be used.
func csrfHandler() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// Put header names we use for CSRF recognition into Vary response header.
			if setting.CORSConfig.Enabled {
				resp.Header().Set("Vary", "Origin, Sec-Fetch-Site")
			} else {
				resp.Header().Set("Vary", "Sec-Fetch-Site")
			}

			// Allow requests not recognized as CSRF.
			secFetchSite := strings.ToLower(req.Header.Get("Sec-Fetch-Site"))
			if req.Method == "GET" || // GET, HEAD and OPTIONS must not be used for changing state (CSRF resistant).
				req.Method == "HEAD" ||
				req.Method == "OPTIONS" ||
				secFetchSite == "" || // Accept requests from clients without Fetch Metadata Request Headers support.
				secFetchSite == "same-origin" || // Accept requests from own origin.
				secFetchSite == "none" || // Accept requests initiated by user (i.e. using bookmark).
				((secFetchSite == "same-site" || secFetchSite == "cross-site") && // Accept cross site requests allowed by CORS.
					setting.CORSConfig.Enabled && setting.Cors.OriginAllowed(req)) {
				next.ServeHTTP(resp, req)
				return
			}

			// Forbid and log other requests as CSRF.
			log.Error("CSRF rejected: METHOD=\"%s\", Origin=\"%s\", Sec-Fetch-Site=\"%s\"", req.Method, req.Header.Get("Origin"), secFetchSite)
			http.Error(resp, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		})
	}
}
