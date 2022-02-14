// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessageLogger(t *testing.T) {
	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname
	maxLength := 10

	ml := NewMessageLogger()
	ml.Init(fmt.Sprintf("{\"expression\":\"FILENAME\",\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"max_length\":%d,\"colorize\":false}", prefix, level.String(), flags, maxLength))
	defer ml.Close()

	realML := ml.(*MessageLogger)

	messageLogLock.Lock()
	messageLog := messageLogRegistry["common"]
	messageLogLock.Unlock()

	assert.Equal(t, flags, realML.Flags)
	assert.Equal(t, level, realML.Level)
	assert.Equal(t, level, ml.GetLevel())

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	dateString := date.UTC().Format("2006/01/02 15:04:05")

	fnEvent := func(i int) (Event, string) {
		event := Event{
			level:    INFO,
			msg:      fmt.Sprintf("TEST MSG: %d", i),
			caller:   "CALLER",
			filename: "FULL/FILENAME",
			line:     i,
			time:     date,
		}

		expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
		return event, expected
	}

	expectedMessages := make([]string, 10)

	for i := 0; i < 97; i++ {
		event, expected := fnEvent(i)

		ml.LogEvent(&event)

		expectedMessages[i%10] = expected

		messages := messageLog.Get()

		assert.Equal(t, expected, messages[len(messages)-1])
		if i < 10 {
			assert.Equal(t, len(messages), i+1)
			assert.Equal(t, expectedMessages[0], messages[0])
		} else {
			assert.Equal(t, len(messages), 10)
			assert.Equal(t, expectedMessages[(i+1)%10], messages[0])
		}
	}

	messageLog.Resize(11)
	messages := messageLog.Get()
	_, expected := fnEvent(96)
	assert.Equal(t, expected, messages[len(messages)-1])
	assert.Equal(t, len(messages), 10)
	assert.Equal(t, expectedMessages[7], messages[0])
	event, expected := fnEvent(96)
	ml.LogEvent(&event)
	messages = messageLog.Get()
	assert.Equal(t, expected, messages[len(messages)-1])
	assert.Equal(t, len(messages), 11)
	assert.Equal(t, expectedMessages[7], messages[0])
}
