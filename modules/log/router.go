// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
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
		m.Use(func(ctx *macaron.Context) {
			start := time.Now()

			Log(0, level, "Started %s %s for %s", ctx.Req.Method, ctx.Req.RequestURI, ctx.RemoteAddr())

			rw := ctx.Resp.(macaron.ResponseWriter)
			ctx.Next()

			content := fmt.Sprintf("Completed %s %s %v %s in %v", ctx.Req.Method, ctx.Req.RequestURI, rw.Status(), http.StatusText(rw.Status()), time.Since(start))
			if ColorLog {
				switch rw.Status() {
				case 200, 201, 202:
					content = fmt.Sprintf("\033[1;32m%s\033[0m", content)
				case 301, 302:
					content = fmt.Sprintf("\033[1;37m%s\033[0m", content)
				case 304:
					content = fmt.Sprintf("\033[1;33m%s\033[0m", content)
				case 401, 403:
					content = fmt.Sprintf("\033[4;31m%s\033[0m", content)
				case 404:
					content = fmt.Sprintf("\033[1;31m%s\033[0m", content)
				case 500:
					content = fmt.Sprintf("\033[1;36m%s\033[0m", content)
				}
			}
			Log(0, level, content)

		})
	}
}
