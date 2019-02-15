// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"net/smtp"
	"strings"
)

const (
	subjectPhrase = "Diagnostic message from server"
)

type smtpWriter struct {
	owner *SMTPLogger
}

// Write sends the message as an email
func (s smtpWriter) Write(p []byte) (int, error) {
	return s.owner.sendMail(p)
}

// Close does nothing
func (s smtpWriter) Close() error {
	return nil
}

// SMTPLogger implements LoggerInterface and is used to send emails via given SMTP-server.
type SMTPLogger struct {
	BaseLogger
	Username           string   `json:"Username"`
	Password           string   `json:"password"`
	Host               string   `json:"host"`
	Subject            string   `json:"subject"`
	RecipientAddresses []string `json:"sendTos"`
	sendMailFn         func(string, smtp.Auth, string, []string, []byte) error
}

// NewSMTPLogger creates smtp writer.
func NewSMTPLogger() LoggerInterface {
	s := &SMTPLogger{}
	s.Level = TRACE
	s.sendMailFn = smtp.SendMail
	return s
}

// Init smtp writer with json config.
// config like:
//	{
//		"Username":"example@gmail.com",
//		"password:"password",
//		"host":"smtp.gmail.com:465",
//		"subject":"email title",
//		"sendTos":["email1","email2"],
//		"level":LevelError
//	}
func (sw *SMTPLogger) Init(jsonconfig string) error {
	err := json.Unmarshal([]byte(jsonconfig), sw)
	if err != nil {
		return err
	}
	sw.createLogger(smtpWriter{
		owner: sw,
	})
	sw.sendMailFn = smtp.SendMail
	return nil
}

// WriteMsg writes message in smtp writer.
// it will send an email with subject and only this message.
func (sw *SMTPLogger) sendMail(p []byte) (int, error) {
	hp := strings.Split(sw.Host, ":")

	// Set up authentication information.
	auth := smtp.PlainAuth(
		"",
		sw.Username,
		sw.Password,
		hp[0],
	)
	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	contentType := "Content-Type: text/plain" + "; charset=UTF-8"
	mailmsg := []byte("To: " + strings.Join(sw.RecipientAddresses, ";") + "\r\nFrom: " + sw.Username + "<" + sw.Username +
		">\r\nSubject: " + sw.Subject + "\r\n" + contentType + "\r\n\r\n")
	mailmsg = append(mailmsg, p...)
	return len(p), sw.sendMailFn(
		sw.Host,
		auth,
		sw.Username,
		sw.RecipientAddresses,
		mailmsg,
	)
}

// Flush when log should be flushed
func (sw *SMTPLogger) Flush() {
}

func init() {
	Register("smtp", NewSMTPLogger)
}
