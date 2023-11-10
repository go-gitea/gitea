// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"time"
)

var statusToColor = map[int][]ColorAttribute{
	100: {Bold},
	200: {FgGreen},
	300: {FgYellow},
	304: {FgCyan},
	400: {Bold, FgRed},
	401: {Bold, FgMagenta},
	403: {Bold, FgMagenta},
	500: {Bold, BgRed},
}

// ColoredStatus adds colors for HTTP status
func ColoredStatus(status int, s ...string) *ColoredValue {
	color, ok := statusToColor[status]
	if !ok {
		color, ok = statusToColor[(status/100)*100]
	}
	if !ok {
		color = []ColorAttribute{Bold}
	}
	if len(s) > 0 {
		return NewColoredValue(s[0], color...)
	}
	return NewColoredValue(status, color...)
}

var methodToColor = map[string][]ColorAttribute{
	"GET":    {FgBlue},
	"POST":   {FgGreen},
	"DELETE": {FgRed},
	"PATCH":  {FgCyan},
	"PUT":    {FgYellow, Faint},
	"HEAD":   {FgBlue, Faint},
}

// ColoredMethod adds colors for HTTP methods on log
func ColoredMethod(method string) *ColoredValue {
	color, ok := methodToColor[method]
	if !ok {
		return NewColoredValue(method, Bold)
	}
	return NewColoredValue(method, color...)
}

var (
	durations = []time.Duration{
		10 * time.Millisecond,
		100 * time.Millisecond,
		1 * time.Second,
		5 * time.Second,
		10 * time.Second,
	}

	durationColors = [][]ColorAttribute{
		{FgGreen},
		{Bold},
		{FgYellow},
		{FgRed, Bold},
		{BgRed},
	}

	wayTooLong = BgMagenta
)

// ColoredTime converts the provided time to a ColoredValue for logging. The duration is always formatted in milliseconds.
func ColoredTime(duration time.Duration) *ColoredValue {
	// the output of duration in Millisecond is more readable:
	// * before: "100.1ms" "100.1μs" "100.1s"
	// * better: "100.1ms" "0.1ms"   "100100.0ms", readers can compare the values at first glance.
	str := fmt.Sprintf("%.1fms", float64(duration.Microseconds())/1000)
	for i, k := range durations {
		if duration < k {
			return NewColoredValue(str, durationColors[i]...)
		}
	}
	return NewColoredValue(str, wayTooLong)
}
