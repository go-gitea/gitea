// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSMTPLogger(t *testing.T) {
	opt := WriterSMTPOption{
		Username:           "user",
		Password:           "pass",
		Host:               "host",
		Subject:            "subject",
		RecipientAddresses: []string{"u@h.com"},
	}
	w := NewEventWriterSMTP("test", WriterMode{WriterOption: opt})
	s := w.(*eventWriterSMTP)

	var envToHost string
	var envFrom string
	var envTo []string
	var envMsg []byte
	s.smtpWriter.sendMailFn = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		envToHost = addr
		envFrom = from
		envTo = to
		envMsg = msg
		return nil
	}

	_, err := s.smtpWriter.Write([]byte("test msg"))
	assert.NoError(t, err)
	assert.Equal(t, opt.Host, envToHost)
	assert.Equal(t, opt.Username, envFrom)
	assert.Equal(t, opt.RecipientAddresses, envTo)
	assert.Contains(t, string(envMsg), "test msg")
}
