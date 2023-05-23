// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"net/smtp"
	"strings"
)

type WriterSMTPOption struct {
	Username           string
	Password           string
	Host               string
	Subject            string
	RecipientAddresses []string
}

type eventWriterSMTP struct {
	*EventWriterBaseImpl
	smtpWriter smtpWriter
}

var _ EventWriter = (*eventWriterSMTP)(nil)

func NewEventWriterSMTP(writerName string, writerMode WriterMode) EventWriter {
	w := &eventWriterSMTP{EventWriterBaseImpl: NewEventWriterBase(writerName, "smtp", writerMode)}
	w.smtpWriter = smtpWriter{
		opt:        writerMode.WriterOption.(WriterSMTPOption),
		sendMailFn: smtp.SendMail,
	}
	w.OutputWriteCloser = &w.smtpWriter
	return w
}

func init() {
	RegisterEventWriter("smtp", NewEventWriterSMTP)
}

type smtpWriter struct {
	opt        WriterSMTPOption
	sendMailFn func(string, smtp.Auth, string, []string, []byte) error
}

func (s *smtpWriter) Close() error {
	return nil
}

// below is copied from old code

func (s *smtpWriter) Write(p []byte) (int, error) {
	hp := strings.Split(s.opt.Host, ":")
	auth := smtp.PlainAuth("", s.opt.Username, s.opt.Password, hp[0])
	mailMsg := []byte("To: " + strings.Join(s.opt.RecipientAddresses, ";") + "\r\n" +
		"From: " + s.opt.Username + "<" + s.opt.Username + ">" + "\r\n" +
		"Subject: " + s.opt.Subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8" + "\r\n" +
		"\r\n",
	)
	mailMsg = append(mailMsg, p...)
	return len(p), s.sendMailFn(s.opt.Host, auth, s.opt.Username, s.opt.RecipientAddresses, mailMsg)
}
