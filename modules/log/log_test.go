// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func baseConsoleTest(t *testing.T, logger *MultiChannelledLogger) (chan []byte, chan bool) {
	written := make(chan []byte)
	closed := make(chan bool)

	c := CallbackWriteCloser{
		callback: func(p []byte, close bool) {
			written <- p
			closed <- close
		},
	}
	m := logger.MultiChannelledLog

	channelledLog := m.GetEventLogger("console")
	assert.NotEmpty(t, channelledLog)
	realChanLog, ok := channelledLog.(*ChannelledLog)
	assert.True(t, ok)
	realCL, ok := realChanLog.loggerProvider.(*ConsoleLogger)
	assert.True(t, ok)
	assert.Equal(t, INFO, realCL.Level)
	realCL.out = c

	format := "test: %s"
	args := []interface{}{"A"}

	logger.Log(0, INFO, format, args...)
	line := <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.False(t, <-closed)

	format = "test2: %s"
	logger.Warn(format, args...)
	line = <-written

	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.False(t, <-closed)

	format = "testerror: %s"
	logger.Error(format, args...)
	line = <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.False(t, <-closed)
	return written, closed
}

func TestNewLoggerUnexported(t *testing.T) {
	level := INFO
	logger := newLogger("UNEXPORTED", 0)
	err := logger.SetLogger("console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))
	assert.NoError(t, err)
	out := logger.MultiChannelledLog.GetEventLogger("console")
	assert.NotEmpty(t, out)
	chanlog, ok := out.(*ChannelledLog)
	assert.True(t, ok)
	assert.Equal(t, "console", chanlog.provider)
	assert.Equal(t, INFO, logger.GetLevel())
	baseConsoleTest(t, logger)
}

func TestNewLoggger(t *testing.T) {
	level := INFO
	logger := NewLogger(0, "console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))

	assert.Equal(t, INFO, GetLevel())
	assert.False(t, IsTrace())
	assert.False(t, IsDebug())
	assert.True(t, IsInfo())
	assert.True(t, IsWarn())
	assert.True(t, IsError())

	written, closed := baseConsoleTest(t, logger)

	format := "test: %s"
	args := []interface{}{"A"}

	Log(0, INFO, format, args...)
	line := <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.False(t, <-closed)

	Info(format, args...)
	line = <-written
	assert.Contains(t, string(line), fmt.Sprintf(format, args...))
	assert.False(t, <-closed)

	go DelLogger("console")
	line = <-written
	assert.Equal(t, "", string(line))
	assert.True(t, <-closed)
}

func TestNewLogggerRecreate(t *testing.T) {
	level := INFO
	NewLogger(0, "console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))

	assert.Equal(t, INFO, GetLevel())
	assert.False(t, IsTrace())
	assert.False(t, IsDebug())
	assert.True(t, IsInfo())
	assert.True(t, IsWarn())
	assert.True(t, IsError())

	format := "test: %s"
	args := []interface{}{"A"}

	Log(0, INFO, format, args...)

	NewLogger(0, "console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))

	assert.Equal(t, INFO, GetLevel())
	assert.False(t, IsTrace())
	assert.False(t, IsDebug())
	assert.True(t, IsInfo())
	assert.True(t, IsWarn())
	assert.True(t, IsError())

	Log(0, INFO, format, args...)

	assert.Panics(t, func() {
		NewLogger(0, "console", "console", fmt.Sprintf(`{"level":"%s"`, level.String()))
	})

	go DelLogger("console")

	// We should be able to redelete without a problem
	go DelLogger("console")
}

func TestNewNamedLogger(t *testing.T) {
	level := INFO
	err := NewNamedLogger("test", 0, "console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))
	assert.NoError(t, err)
	logger, _ := NamedLoggers.Load("test")
	assert.Equal(t, level, logger.GetLevel())

	written, closed := baseConsoleTest(t, logger)
	go DelNamedLogger("test")
	line := <-written
	assert.Equal(t, "", string(line))
	assert.True(t, <-closed)
}
