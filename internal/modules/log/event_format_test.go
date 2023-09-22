// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestItoa(t *testing.T) {
	b := itoa(nil, 0, 0)
	assert.Equal(t, "0", string(b))

	b = itoa(nil, 0, 1)
	assert.Equal(t, "0", string(b))

	b = itoa(nil, 0, 2)
	assert.Equal(t, "00", string(b))
}

func TestEventFormatTextMessage(t *testing.T) {
	res := EventFormatTextMessage(&WriterMode{Prefix: "[PREFIX] ", Colorize: false, Flags: Flags{defined: true, flags: 0xffffffff}},
		&Event{
			Time:         time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
			Caller:       "caller",
			Filename:     "filename",
			Line:         123,
			GoroutinePid: "pid",
			Level:        ERROR,
			Stacktrace:   "stacktrace",
		},
		"msg format: %v %v", "arg0", NewColoredValue("arg1", FgBlue),
	)

	assert.Equal(t, `[PREFIX] 2020/01/02 03:04:05.000000 filename:123:caller [E] [pid] msg format: arg0 arg1
	stacktrace

`, string(res))

	res = EventFormatTextMessage(&WriterMode{Prefix: "[PREFIX] ", Colorize: true, Flags: Flags{defined: true, flags: 0xffffffff}},
		&Event{
			Time:         time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
			Caller:       "caller",
			Filename:     "filename",
			Line:         123,
			GoroutinePid: "pid",
			Level:        ERROR,
			Stacktrace:   "stacktrace",
		},
		"msg format: %v %v", "arg0", NewColoredValue("arg1", FgBlue),
	)

	assert.Equal(t, "[PREFIX] \x1b[36m2020/01/02 03:04:05.000000 \x1b[0m\x1b[32mfilename:123:\x1b[32mcaller\x1b[0m \x1b[1;31m[E]\x1b[0m [\x1b[93mpid\x1b[0m] msg format: arg0 \x1b[34marg1\x1b[0m\n\tstacktrace\n\n", string(res))
}
