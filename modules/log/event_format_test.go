// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestItoa(t *testing.T) {
	for width, expected := range []string{"0", "0", "00"} {
		assert.Equal(t, expected, string(itoa(nil, 0, width)))
	}
}

func TestEventFormatTextMessage(t *testing.T) {
	for _, testCase := range []struct {
		colorize bool
		expected string
	}{
		{false, `[PREFIX] 2020/01/02 03:04:05.000000 filename:123:caller [E] [no-gopid] msg format: arg0 arg1
	stacktrace

`},
		{true, "[PREFIX] \x1b[36m2020/01/02 03:04:05.000000 \x1b[0m\x1b[32mfilename:123:\x1b[32mcaller\x1b[0m \x1b[1;31m[E]\x1b[0m [\x1b[93mno-gopid\x1b[0m] msg format: arg0 \x1b[34marg1\x1b[0m\n\tstacktrace\n\n"},
	} {
		res := EventFormatTextMessage(&WriterMode{Prefix: "[PREFIX] ", Colorize: testCase.colorize, Flags: Flags{defined: true, flags: 0xffffffff}},
			&Event{
				Time:       time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
				Caller:     "caller",
				Filename:   "filename",
				Line:       123,
				Level:      ERROR,
				Stacktrace: "stacktrace",
			},
			"msg format: %v %v", "arg0", NewColoredValue("arg1", FgBlue),
		)
		assert.Equal(t, testCase.expected, string(res))
	}
}
