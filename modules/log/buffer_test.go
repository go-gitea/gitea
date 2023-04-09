// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBufferLogger(t *testing.T) {
	logger := NewBufferLogger()
	bufferLogger := logger.(*BufferLogger)
	assert.NotNil(t, bufferLogger)

	err := logger.Init("")
	assert.NoError(t, err)

	location, _ := time.LoadLocation("EST")
	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	msg := "TEST MSG"
	event := Event{
		level:    INFO,
		msg:      msg,
		caller:   "CALLER",
		filename: "FULL/FILENAME",
		line:     1,
		time:     date,
	}
	logger.LogEvent(&event)
	content, err := bufferLogger.Content()
	assert.NoError(t, err)
	assert.Contains(t, content, msg)
	logger.Close()
}

func TestBufferLoggerContent(t *testing.T) {
	level := INFO
	logger := NewLogger(0, "console", "console", fmt.Sprintf(`{"level":"%s"}`, level.String()))

	logger.SetLogger("buffer", "buffer", "{}")
	defer logger.DelLogger("buffer")

	msg := "A UNIQUE MESSAGE"
	Error(msg)

	found := false
	for i := 0; i < 30000; i++ {
		content, err := logger.GetLoggerProviderContent("buffer")
		assert.NoError(t, err)
		if strings.Contains(content, msg) {
			found = true
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
	assert.True(t, found)
}
