// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web/types"
)

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
	logRequest := func(level log.Level, fmt string, args ...any) {
		logger.Log(2, &log.Event{Level: level, Caller: callerName}, fmt, args...)
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
			logRequest(log.TRACE, "router: %s %v %s for %s", startMessage, log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr)
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
			logLevel := log.WARN
			if isLongPolling {
				logLevel = log.INFO
				message = pollingMessage
			}
			logRequest(logLevel, "router: %s %v %s for %s, elapsed %v @ %s",
				message,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(record.startTime)),
				handlerFuncInfo,
			)
			return
		}

		if panicErr != nil {
			logRequest(log.WARN, "router: %s %v %s for %s, panic in %v @ %s, err=%v",
				failedMessage,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(record.startTime)),
				handlerFuncInfo,
				panicErr,
			)
			return
		}

		var status int
		if v, ok := record.respWriter.(types.ResponseStatusProvider); ok {
			status = v.WrittenStatus()
		}
		logLevel := record.logLevel
		if logLevel == log.UNDEFINED {
			logLevel = log.INFO
		}
		// lower the log level for some specific requests, in most cases these logs are not useful
		if status > 0 && status < 400 &&
			req.RequestURI == "/api/actions/runner.v1.RunnerService/FetchTask" /* Actions Runner polling */ {
			logLevel = log.TRACE
		}
		message := completedMessage
		if isUnknownHandler {
			logLevel = log.ERROR
			message = unknownHandlerMessage
		}

		logRequest(logLevel, "router: %s %v %s for %s, %v %v in %v @ %s",
			message,
			log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
			log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(record.startTime)),
			handlerFuncInfo,
		)
	}
}
