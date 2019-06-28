// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSMTPLogger(t *testing.T) {
	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname
	username := "testuser"
	password := "testpassword"
	host := "testhost"
	subject := "testsubject"
	sendTos := []string{"testto1", "testto2"}

	logger := NewSMTPLogger()
	smtpLogger, ok := logger.(*SMTPLogger)
	assert.Equal(t, true, ok)

	err := logger.Init(fmt.Sprintf("{\"prefix\":\"%s\",\"level\":\"%s\",\"flags\":%d,\"username\":\"%s\",\"password\":\"%s\",\"host\":\"%s\",\"subject\":\"%s\",\"sendTos\":[\"%s\",\"%s\"]}", prefix, level.String(), flags, username, password, host, subject, sendTos[0], sendTos[1]))
	assert.NoError(t, err)

	assert.Equal(t, flags, smtpLogger.Flags)
	assert.Equal(t, level, smtpLogger.Level)
	assert.Equal(t, level, logger.GetLevel())

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

	expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)

	var envToHost string
	var envFrom string
	var envTo []string
	var envMsg []byte
	smtpLogger.sendMailFn = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		envToHost = addr
		envFrom = from
		envTo = to
		envMsg = msg
		return nil
	}

	err = logger.LogEvent(&event)
	assert.NoError(t, err)
	assert.Equal(t, host, envToHost)
	assert.Equal(t, username, envFrom)
	assert.Equal(t, sendTos, envTo)
	assert.Contains(t, string(envMsg), expected)

	logger.Flush()

	event.level = WARN
	expected = fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.filename, event.line, event.caller, strings.ToUpper(event.level.String())[0], event.msg)
	err = logger.LogEvent(&event)
	assert.NoError(t, err)
	assert.Equal(t, host, envToHost)
	assert.Equal(t, username, envFrom)
	assert.Equal(t, sendTos, envTo)
	assert.Contains(t, string(envMsg), expected)

	logger.Close()
}
