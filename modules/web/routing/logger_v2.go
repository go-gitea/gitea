// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routing

import (
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// NewLoggerHandlerV2 is a handler that will log routing to the router log taking account of
// routing information
func NewLoggerHandlerV2() func(next http.Handler) http.Handler {
	manager := requestRecordsManager{
		requestRecords: map[uint64]*requestRecord{},
	}
	manager.startSlowQueryDetector(3 * time.Second)

	logger := log.GetLogger("router")
	manager.print = logPrinter(logger)
	return manager.handler
}

func logPrinter(logger log.Logger) func(trigger Event, record *requestRecord) {
	return func(trigger Event, record *requestRecord) {
		if trigger == StartEvent && setting.LogLevel > log.DEBUG {
			// for performance, if the "started" message shouldn't be logged, we just return as early as possible
			// developers could set both `log.LEVEL=Debug` to get the "started" request messages.
			return
		}

		shortFilename := ""
		line := 0
		shortName := ""

		record.lock.RLock()
		isLongPolling := record.isLongPolling
		if record.funcInfo != nil {
			shortFilename, line, shortName = record.funcInfo.shortFile, record.funcInfo.line, record.funcInfo.shortName
		} else {
			// we might not find all handlers, so if a handler has not called `UpdateFuncInfo`, we won't know its information
			// in such case, we should debug to find what handler it is and use `UpdateFuncInfo` to report its information
			shortFilename = "unknown-handler"
		}
		record.lock.RUnlock()

		req := record.request

		if trigger == StartEvent {
			// when a request starts, we have no information about the handler function information, we only have the request path
			logger.Debug("router: started %v %s for %s", log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr)
			return
		}

		handlerFuncInfo := fmt.Sprintf("%s:%d(%s)", shortFilename, line, shortName)
		if trigger == StillExecutingEvent {
			message := "still-executing"
			level := log.WARN
			if isLongPolling {
				level = log.INFO
				message = "long-polling"
			}
			_ = logger.Log(0, level, "router: %s %v %s for %s, elapsed %v @ %s",
				message,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(record.startTime)),
				handlerFuncInfo,
			)
		} else {
			if record.panicError != nil {
				_ = logger.Log(0, log.WARN, "router: failed %v %s for %s, panic in %v @ %s, err=%v",
					shortFilename, line, shortName,
					log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
					log.ColoredTime(time.Since(record.startTime)),
					handlerFuncInfo,
					record.panicError,
				)
			} else {
				var status int
				if v, ok := record.responseWriter.(context.ResponseWriter); ok {
					status = v.Status()
				}
				_ = logger.Log(0, setting.RouterLogLevel, "router: completed %v %s for %s, %v %v in %v @ %s",
					log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
					log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(record.startTime)),
					handlerFuncInfo,
				)
			}
		}
	}
}
