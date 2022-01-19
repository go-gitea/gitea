// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/chi-middleware/proxy"
	"github.com/go-chi/chi/v5/middleware"
)

// Middlewares returns common middlewares
func Middlewares() []func(http.Handler) http.Handler {
	var handlers = []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				// First of all escape the URL RawPath to ensure that all routing is done using a correctly escaped URL
				req.URL.RawPath = req.URL.EscapedPath()

				ctx, _, finished := process.GetManager().AddContext(req.Context(), fmt.Sprintf("%s: %s", req.Method, req.RequestURI))
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

	if !setting.DisableRouterLog && setting.RouterLogLevel != log.NONE {
		if log.GetLogger("router").GetLevel() <= setting.RouterLogLevel {
			handlers = append(handlers, LoggerHandler(setting.RouterLogLevel))
		}
	}
	if setting.EnableAccessLog {
		handlers = append(handlers, context.AccessLogger())
	}

	handlers = append(handlers, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// Why we need this? The Recovery() will try to render a beautiful
			// error page for user, but the process can still panic again, and other
			// middleware like session also may panic then we have to recover twice
			// and send a simple error page that should not panic any more.
			defer func() {
				if err := recover(); err != nil {
					combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, string(log.Stack(2)))
					log.Error("%v", combinedErr)
					if setting.IsProd {
						http.Error(resp, http.StatusText(500), 500)
					} else {
						http.Error(resp, combinedErr, 500)
					}
				}
			}()
			next.ServeHTTP(resp, req)
		})
	})
	return handlers
}
