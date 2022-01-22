// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routing

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// NewLoggerHandler is a handler that will log routing to the router log taking account of
// routing information
func NewLoggerHandler() func(next http.Handler) http.Handler {
	manager := requestRecordsManager{
		requestRecords: map[uint64]*requestRecord{},
	}
	manager.startSlowQueryDetector(3 * time.Second)

	logger := log.GetLogger("router")
	manager.print = logPrinter(logger)
	return manager.handler
}

var (
	startMessage          = log.NewColoredValueBytes("started  ", log.DEBUG.Color())
	slowMessage           = log.NewColoredValueBytes("slow     ", log.WARN.Color())
	pollingMessage        = log.NewColoredValueBytes("polling  ", log.INFO.Color())
	failedMessage         = log.NewColoredValueBytes("failed   ", log.WARN.Color())
	completedMessage      = log.NewColoredValueBytes("completed", log.INFO.Color())
	unknownHandlerMessage = log.NewColoredValueBytes("completed", log.ERROR.Color())
)

func logPrinter(logger log.Logger) func(trigger Event, record *requestRecord) {
	return func(trigger Event, record *requestRecord) {
		if trigger == StartEvent {
			if !logger.IsTrace() {
				// for performance, if the "started" message shouldn't be logged, we just return as early as possible
				// developers can set the router log level to TRACE to get the "started" request messages.
				return
			}
			// when a request starts, we have no information about the handler function information, we only have the request path
			req := record.request
			logger.Trace("router: %s %v %s for %s", startMessage, log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr)
			return
		}

		req := record.request

		// Get data from the record
		record.lock.Lock()
		handlerFuncInfo := record.funcInfo.String()
		isLongPolling := record.isLongPolling
		isUnknownHandler := record.funcInfo == nil
		panicErr := record.panicError
		record.lock.Unlock()

		if trigger == StillExecutingEvent {
			message := slowMessage
			level := log.WARN
			if isLongPolling {
				level = log.INFO
				message = pollingMessage
			}
			_ = logger.Log(0, level, "router: %s %v %s for %s, elapsed %v @ %s",
				message,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(record.startTime)),
				handlerFuncInfo,
			)
			return
		}

		if panicErr != nil {
			_ = logger.Log(0, log.WARN, "router: %s %v %s for %s, panic in %v @ %s, err=%v",
				failedMessage,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(record.startTime)),
				handlerFuncInfo,
				panicErr,
			)
			return
		}

		var status int
		if v, ok := record.responseWriter.(context.ResponseWriter); ok {
			status = v.Status()
		}
		level := log.INFO
		message := completedMessage
		if isUnknownHandler {
			level = log.ERROR
			message = unknownHandlerMessage
		}

		_ = logger.Log(0, level, "router: %s %v %s for %s, %v %v in %v @ %s",
			message,
			log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
			log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(record.startTime)),
			handlerFuncInfo,
		)
	}
}
