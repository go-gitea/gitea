// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"net/http"
	"time"

	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// NewLoggerHandlerV2 is a handler that will log the routing to the default gitea log
// About performance:
// In v1, every request outputs 2 logs (Started/Completed)
// In v2 (this), every request only outputs one log,
//    all runtime reflections of handler functions are cached
//	  the mutexes work in fast path for most cases (atomic incr) because there is seldom concurrency writings.
// So generally speaking, the `logger context` doesn't cost much, using v2 with `logger context` will not affect performance
// Instead, the performance may be improved because now only 1 log is outputted for each request.
func NewLoggerHandlerV2() func(next http.Handler) http.Handler {
	lh := logContextHandler{
		requestRecordMap: map[uint64]*logRequestRecord{},
	}
	lh.startSlowQueryDetector(3 * time.Second)

	lh.printLog = func(trigger LogRequestTrigger, reqRec *logRequestRecord) {
		if trigger == LogRequestStart && !log.DEBUG.IsEnabledOn(setting.RouterLogLevel) {
			// for performance, if the START message shouldn't be logged, we just return as early as possible
			// developers could set both `log.LEVEL=debug` and `log.ROUTER_LOG_LEVEL=debug` to get the "started" request messages.
			return
		}

		funcFileShort := ""
		funcLine := 0
		funcNameShort := ""
		reqRec.funcInfoMu.RLock()
		isLongPolling := reqRec.isLongPolling
		if reqRec.funcInfo != nil {
			funcFileShort, funcLine, funcNameShort = reqRec.funcInfo.funcFileShort, reqRec.funcInfo.funcLine, reqRec.funcInfo.funcNameShort
		} else {
			// we might not find all handlers, so if a handler is not processed by our `UpdateContextHandlerFuncInfo`, we won't know its information
			// in such case, we should debug to find what handler it is and use `UpdateContextHandlerFuncInfo` to report its information
			funcFileShort = "unknown-handler"
		}
		reqRec.funcInfoMu.RUnlock()

		logger := log.GetLogger("router")
		req := reqRec.httpRequest
		if trigger == LogRequestStart {
			// when a request starts, we have no information about the handler function information, we only have the request path
			_ = logger.Log(0, log.DEBUG, "router: started %v %s for %s", log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr)
			return
		}

		handlerFuncInfo := fmt.Sprintf("%s:%d(%s)", funcFileShort, funcLine, funcNameShort)
		if trigger == LogRequestExecuting {
			message := "still-executing"
			level := log.WARN
			if isLongPolling {
				level = log.INFO
				message = "long-polling"
			}
			_ = logger.Log(0, level, "router: %s %v %s for %s, elapsed %v @ %s",
				message,
				log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
				log.ColoredTime(time.Since(reqRec.startTime)),
				handlerFuncInfo,
			)
		} else {
			if reqRec.panicError != nil {
				_ = logger.Log(0, log.WARN, "router: failed %v %s for %s, panic in %v @ %s, err=%v",
					funcFileShort, funcLine, funcNameShort,
					log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
					log.ColoredTime(time.Since(reqRec.startTime)),
					handlerFuncInfo,
					reqRec.panicError,
				)
			} else {
				var status int
				if v, ok := reqRec.responseWriter.(gitea_context.ResponseWriter); ok {
					status = v.Status()
				}
				_ = logger.Log(0, log.INFO, "router: completed %v %s for %s, %v %v in %v @ %s",
					log.ColoredMethod(req.Method), req.RequestURI, req.RemoteAddr,
					log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(reqRec.startTime)),
					handlerFuncInfo,
				)
			}
		}
	}

	return lh.handler
}
