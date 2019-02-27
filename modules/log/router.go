// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"net/http"
	"runtime"
	"time"

	macaron "gopkg.in/macaron.v1"
)

// ColorLog sets this logger to print in color
var (
	ColorLog = true
)

func init() {
	ColorLog = runtime.GOOS != "windows"
}

// SetupRouterLogger will setup macaron to routing to the main gitea log
func SetupRouterLogger(m *macaron.Macaron, level Level) {
	if GetLevel() <= level {
		m.Use(RouterHandler)
	}
}

// RouterHandler is a macaron handler that will log the routing to the default gitea log
func RouterHandler(ctx *macaron.Context) {
	start := time.Now()

	Log(0, level, "Started %s %s for %s", ctx.Req.Method, ctx.Req.RequestURI, ctx.RemoteAddr())

	rw := ctx.Resp.(macaron.ResponseWriter)
	ctx.Next()

	color := ""
	reset := ""
	if ColorLog {
		reset = "\033[0m"
		switch rw.Status() {
		case 200, 201, 202:
			color = "\033[1;32m"
		case 301, 302:
			color = "\033[1;37m"
		case 304:
			color = "\033[1;33m"
		case 401, 403:
			color = "\033[4;31m"
		case 404:
			color = "\033[1;31m"
		case 500:
			color = "\033[1;36m"
		}
	}
	Log(0, level, "%sCompleted %s %s %v %s in %v%s", color, ctx.Req.Method, ctx.Req.RequestURI, rw.Status(), http.StatusText(rw.Status()), time.Since(start), reset)
}
