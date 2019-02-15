// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type CallbackWriteCloser struct {
	callback func([]byte, bool)
}

func (c CallbackWriteCloser) Write(p []byte) (int, error) {
	c.callback(p, false)
	return len(p), nil
}

func (c CallbackWriteCloser) Close() error {
	c.callback(nil, true)
	return nil
}

func TestBaseLogger(t *testing.T) {
	var written []byte
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written = p
			closed = close
		},
	}
	prefix := "TestPrefix "
	b := BaseLogger{
		out:    c,
		Level:  INFO,
		Flags:  LstdFlags | LUTC,
		Prefix: prefix,
	}
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

	assert.Equal(t, INFO, b.GetLevel())

	expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, event.level.String()[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = DEBUG
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)

	event.level = TRACE
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)

	event.level = WARN
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, event.level.String()[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = ERROR
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, event.level.String()[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = CRITICAL
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, event.level.String()[0], event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	b.Close()
	assert.Equal(t, true, closed)
}

func TestBaseLoggerDated(t *testing.T) {
	var written []byte
	var closed bool

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written = p
			closed = close
		},
	}
	prefix := ""
	b := BaseLogger{
		out:    c,
		Level:  WARN,
		Flags:  Ldate | Ltime | Lshortfile | Lfuncname | Llevel,
		Prefix: prefix,
	}

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	dateString := date.Format("2006/01/02 15:04:05")

	event := Event{
		level:    WARN,
		msg:      "TEST MESSAGE TEST",
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}

	assert.Equal(t, WARN, b.GetLevel())

	expected := fmt.Sprintf("%s%s %s:%d:%s [%s] %s\n", prefix, dateString, "FILENAME", event.line, event.caller, event.level.String(), event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = INFO
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = ERROR
	expected = fmt.Sprintf("%s%s %s:%d:%s [%s] %s\n", prefix, dateString, "FILENAME", event.line, event.caller, event.level.String(), event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = DEBUG
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = CRITICAL
	expected = fmt.Sprintf("%s%s %s:%d:%s [%s] %s\n", prefix, dateString, "FILENAME", event.line, event.caller, event.level.String(), event.msg)
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	event.level = TRACE
	expected = ""
	b.LogEvent(&event)
	assert.Equal(t, expected, string(written))
	assert.Equal(t, false, closed)
	written = written[:0]

	b.Close()
	assert.Equal(t, true, closed)
}
