// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// LoggerHandler is a handler that will log the routing to the default gitea log
func LoggerHandler(level log.Level) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			_ = log.GetLogger("router").Log(0, level, "Started %s %s for %s", log.ColoredMethod(req.Method), req.URL.RequestURI(), req.RemoteAddr)

			next.ServeHTTP(w, req)

			var status int
			if v, ok := w.(context.ResponseWriter); ok {
				status = v.Status()
			}

			_ = log.GetLogger("router").Log(0, level, "Completed %s %s %v %s in %v", log.ColoredMethod(req.Method), req.URL.RequestURI(), log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(start)))
		})
	}
}
