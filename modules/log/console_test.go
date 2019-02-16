// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsoleLoggerMinimalConfig(t *testing.T) {
	for _, level := range Levels() {
		var written []byte
		var closed bool

		c := CallbackWriteCloser{
			callback: func(p []byte, close bool) {
				written = p
				closed = close
			},
		}
		prefix := ""
		flags := LstdFlags

		cw := NewConsoleLogger()
		realCW := cw.(*ConsoleLogger)
		realCW.out = c

		cw.Init(fmt.Sprintf("{\"level\":\"%s\"}", level))
		assert.Equal(t, flags, realCW.Flags)
		assert.Equal(t, FromString(level), realCW.Level)
		assert.Equal(t, FromString(level), cw.GetLevel())
		assert.Equal(t, prefix, realCW.Prefix)
		assert.Equal(t, "", string(written))
		assert.Equal(t, false, closed)
	}
}

func TestConsoleLogger(t *testing.T) {
	var written []byte
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written = p
			closed = close
		},
	}
	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC

	cw := NewConsoleLogger()
	realCW := cw.(*ConsoleLogger)
	realCW.out = c

	cw.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d}", prefix, level.String(), flags))

	assert.Equal(t, flags, realCW.Flags)
	assert.Equal(t, level, realCW.Level)
	assert.Equal(t, level, cw.GetLevel())

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	dateString := date.UTC().Format("2006/01/02 15:04:05")

	event := Event{
		level:    INFO,
		msg:      "TEST MSG",
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}

	expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, event.level.String()[0], event.msg)
	if runtime.GOOS != "windows" {
		expected = pre + colors[event.level] + expected + reset
	}
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = DEBUG
	expected = ""
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)

	event.level = TRACE
	expected = ""
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)

	event.level = WARN
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, event.level.String()[0], event.msg)
	if runtime.GOOS != "windows" {
		expected = pre + colors[event.level] + expected + reset
	}
	cw.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	cw.Close()
	assert.Equal(t, true, closed)
}
