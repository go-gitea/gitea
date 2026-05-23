// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// NewRequestInfoHandler is a handler that saves request info into request context.
// If router logger is enabled, it will also print request logs and detect slow requests.
func NewRequestInfoHandler() func(next http.Handler) http.Handler {
	var reqLogger *loggerRequestManager
	if setting.IsRouteLogEnabled() {
		reqLogger = &loggerRequestManager{
			logPrint: logPrinter(log.GetLogger("router")),
		}
		reqLogger.startSlowQueryDetector(3 * time.Second)
	}
	var requestCounter atomic.Uint64
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			record := &requestRecord{
				index:      requestCounter.Add(1),
				startTime:  time.Now(),
				respWriter: w,
			}
			req = req.WithContext(context.WithValue(req.Context(), contextKey, record))
			record.request = req
			if reqLogger != nil {
				end := reqLogger.handleRequestRecord(record)
				defer end()
			}
			next.ServeHTTP(w, req)
		})
	}
}
