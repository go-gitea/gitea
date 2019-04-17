// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"net/http"
	"time"

	macaron "gopkg.in/macaron.v1"
)

var statusToColor = map[int][]byte{
	100: ColorBytes(Bold),
	200: ColorBytes(FgGreen),
	300: ColorBytes(FgYellow),
	304: ColorBytes(FgCyan),
	400: ColorBytes(Bold, FgRed),
	401: ColorBytes(Bold, FgMagenta),
	403: ColorBytes(Bold, FgMagenta),
	500: ColorBytes(Bold, BgRed),
}

func coloredStatus(status int, s ...string) *ColoredValue {
	color, ok := statusToColor[status]
	if !ok {
		color, ok = statusToColor[(status/100)*100]
	}
	if !ok {
		color = fgBoldBytes
	}
	if len(s) > 0 {
		return NewColoredValueBytes(s[0], &color)
	}
	return NewColoredValueBytes(status, &color)
}

var methodToColor = map[string][]byte{
	"GET":    ColorBytes(FgBlue),
	"POST":   ColorBytes(FgGreen),
	"DELETE": ColorBytes(FgRed),
	"PATCH":  ColorBytes(FgCyan),
	"PUT":    ColorBytes(FgYellow, Faint),
	"HEAD":   ColorBytes(FgBlue, Faint),
}

func coloredMethod(method string) *ColoredValue {
	color, ok := methodToColor[method]
	if !ok {
		return NewColoredValueBytes(method, &fgBoldBytes)
	}
	return NewColoredValueBytes(method, &color)
}

var durations = []time.Duration{
	10 * time.Millisecond,
	100 * time.Millisecond,
	1 * time.Second,
	5 * time.Second,
	10 * time.Second,
}

var durationColors = [][]byte{
	ColorBytes(FgGreen),
	ColorBytes(Bold),
	ColorBytes(FgYellow),
	ColorBytes(FgRed, Bold),
	ColorBytes(BgRed),
}

var wayTooLong = ColorBytes(BgMagenta)

func coloredTime(duration time.Duration) *ColoredValue {
	for i, k := range durations {
		if duration < k {
			return NewColoredValueBytes(duration, &durationColors[i])
		}
	}
	return NewColoredValueBytes(duration, &wayTooLong)
}

// SetupRouterLogger will setup macaron to routing to the main gitea log
func SetupRouterLogger(m *macaron.Macaron, level Level) {
	if GetLevel() <= level {
		m.Use(RouterHandler(level))
	}
}

// RouterHandler is a macaron handler that will log the routing to the default gitea log
func RouterHandler(level Level) func(ctx *macaron.Context) {
	return func(ctx *macaron.Context) {
		start := time.Now()

		GetLogger("router").Log(0, level, "Started %s %s for %s", coloredMethod(ctx.Req.Method), ctx.Req.RequestURI, ctx.RemoteAddr())

		rw := ctx.Resp.(macaron.ResponseWriter)
		ctx.Next()

		status := rw.Status()
		GetLogger("router").Log(0, level, "Completed %s %s %v %s in %v", coloredMethod(ctx.Req.Method), ctx.Req.RequestURI, coloredStatus(status), coloredStatus(status, http.StatusText(rw.Status())), coloredTime(time.Since(start)))
	}
}
