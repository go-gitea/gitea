// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

const (
	subjectPhrase = "Diagnostic message from server"
)

// SMTPWriter implements LoggerInterface and is used to send emails via given SMTP-server.
type SMTPWriter struct {
	Username           string   `json:"Username"`
	Password           string   `json:"password"`
	Host               string   `json:"Host"`
	Subject            string   `json:"subject"`
	RecipientAddresses []string `json:"sendTos"`
	Level              int      `json:"level"`
}

// NewSMTPWriter creates smtp writer.
func NewSMTPWriter() LoggerInterface {
	return &SMTPWriter{Level: TRACE}
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
func (sw *SMTPWriter) Init(jsonconfig string) error {
	return json.Unmarshal([]byte(jsonconfig), sw)
}

// WriteMsg writes message in smtp writer.
// it will send an email with subject and only this message.
func (sw *SMTPWriter) WriteMsg(msg string, skip, level int) error {
	if level < sw.Level {
		return nil
	}

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
		">\r\nSubject: " + sw.Subject + "\r\n" + contentType + "\r\n\r\n" + fmt.Sprintf(".%s", time.Now().Format("2006-01-02 15:04:05")) + msg)

	return smtp.SendMail(
		sw.Host,
		auth,
		sw.Username,
		sw.RecipientAddresses,
		mailmsg,
	)
}

// Flush when log should be flushed
func (sw *SMTPWriter) Flush() {
}

// Destroy when writer is destroy
func (sw *SMTPWriter) Destroy() {
}

func init() {
	Register("smtp", NewSMTPWriter)
}
