// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func baseConsoleTest(t *testing.T, logger *Logger) (chan []byte, chan bool) {
	written := make(chan []byte)
	closed := make(chan bool)

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written <- p
			closed <- close
		},
	}

	consoleLogger, ok := logger.outputs.Load("console")
	assert.Equal(t, true, ok)
	realCL, ok := consoleLogger.(*ConsoleLogger)
	assert.Equal(t, true, ok)
	assert.Equal(t, INFO, realCL.Level)
	realCL.out = c

	format := "test: %s"
	args := []interface{}{"A"}

	logger.Log(0, INFO, format, args...)
	line := <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.Equal(t, false, <-closed)

	format = "test2: %s"
	logger.Warn(format, args...)
	line = <-written

	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.Equal(t, false, <-closed)

	format = "testerror: %s"
	logger.Error(0, format, args...)
	line = <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.Equal(t, false, <-closed)
	return written, closed
}

func TestNewLoggerUnexported(t *testing.T) {
	level := INFO
	logger := newLogger(0)
	err := logger.SetLogger("console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))
	assert.NoError(t, err)
	assert.Equal(t, "console", logger.adapter)
	assert.Equal(t, INFO, logger.GetLevel())
	baseConsoleTest(t, logger)
}

func TestNewLoggger(t *testing.T) {
	level := INFO
	logger := NewLogger(0, "console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))

	assert.Equal(t, INFO, GetLevel())
	assert.Equal(t, false, IsTrace())
	assert.Equal(t, false, IsDebug())
	assert.Equal(t, true, IsInfo())
	assert.Equal(t, true, IsWarn())
	assert.Equal(t, true, IsError())

	written, closed := baseConsoleTest(t, logger)

	format := "test: %s"
	args := []interface{}{"A"}

	Log(0, INFO, format, args...)
	line := <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.Equal(t, false, <-closed)

	Info(format, args...)
	line = <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.Equal(t, false, <-closed)

	go DelLogger("console")
	line = <-written
	assert.Equal(t, "", string(line))
	assert.Equal(t, true, <-closed)
}

func TestNewNamedLogger(t *testing.T) {
	level := INFO
	err := NewNamedLogger("test", 0, "console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))
	assert.NoError(t, err)
	logger := NamedLoggers["test"]
	assert.Equal(t, level, logger.GetLevel())

	written, closed := baseConsoleTest(t, logger)
	go DelNamedLogger("test")
	line := <-written
	assert.Equal(t, "", string(line))
	assert.Equal(t, true, <-closed)
}
