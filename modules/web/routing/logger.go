// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web/types"
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
	startMessage          = log.NewColoredValue("started  ", log.DEBUG.ColorAttributes()...)
	slowMessage           = log.NewColoredValue("slow     ", log.WARN.ColorAttributes()...)
	pollingMessage        = log.NewColoredValue("polling  ", log.INFO.ColorAttributes()...)
	failedMessage         = log.NewColoredValue("failed   ", log.WARN.ColorAttributes()...)
	completedMessage      = log.NewColoredValue("completed", log.INFO.ColorAttributes()...)
	unknownHandlerMessage = log.NewColoredValue("completed", log.ERROR.ColorAttributes()...)
)

func logPrinter(logger log.Logger) func(trigger Event, record *requestRecord) {
	const callerName = "HTTPRequest"
	logTrace := func(fmt string, args ...any) {
		logger.Log(2, &log.Event{Level: log.TRACE, Caller: callerName}, fmt, args...)
	}
	logInfo := func(fmt string, args ...any) {
		logger.Log(2, &log.Event{Level: log.INFO, Caller: callerName}, fmt, args...)
	}
	logWarn := func(fmt string, args ...any) {
		logger.Log(2, &log.Event{Level: log.WARN, Caller: callerName}, fmt, args...)
	}
	logError := func(fmt string, args ...any) {
		logger.Log(2, &log.Event{Level: log.ERROR, Caller: callerName}, fmt, args...)
	}
	return func(trigger Event, record *requestRecord) {
		if trigger == StartEvent {
			if !logger.LevelEnabled(log.TRACE) {
				// for performance, if the "started" message shouldn't be logged, we just return as early as possible
				// developers can set the router log level to TRACE to get the "started" request messages.
				return
			}
			// when a request starts, we have no information about the handler function information, we only have the request path
			req := record.request
			logTrace("router: %s %v %s for %s", startMessage, log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr)
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
			logf := logWarn
			if isLongPolling {
				logf = logInfo
				message = pollingMessage
			}
			logf("router: %s %v %s for %s, elapsed %v @ %s",
				message,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(record.startTime)),
				handlerFuncInfo,
			)
			return
		}

		if panicErr != nil {
			logWarn("router: %s %v %s for %s, panic in %v @ %s, err=%v",
				failedMessage,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(record.startTime)),
				handlerFuncInfo,
				panicErr,
			)
			return
		}

		var status int
		if v, ok := record.responseWriter.(types.ResponseStatusProvider); ok {
			status = v.WrittenStatus()
		}
		logf := logInfo
		if strings.HasPrefix(req.RequestURI, "/assets/") {
			logf = logTrace
		}
		message := completedMessage
		if isUnknownHandler {
			logf = logError
			message = unknownHandlerMessage
		}

		logf("router: %s %v %s for %s, %v %v in %v @ %s",
			message,
			log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
			log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(record.startTime)),
			handlerFuncInfo,
		)
	}
}
