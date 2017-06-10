// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"gopkg.in/gomail.v2"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Message mail body and log info
type Message struct {
	*gomail.Message

	Info string // Message information for log purpose.
}

// NewMessageFrom creates new mail message object with custom From header.
func NewMessageFrom(to []string, from, subject, body string) *Message {
	log.Trace("NewMessageFrom (body):\n%s", body)

	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to...)
	msg.SetHeader("Subject", subject)
	msg.SetDateHeader("Date", time.Now())

	plainBody, err := html2text.FromString(body)
	if err != nil || setting.MailService.SendAsPlainText {
		if strings.Contains(body[:100], "<html>") {
			log.Warn("Mail contains HTML but configured to send as plain text.")
		}
		msg.SetBody("text/plain", plainBody)
	} else {
		msg.SetBody("text/plain", plainBody)
		msg.AddAlternative("text/html", body)
	}

	return &Message{
		Message: msg,
	}
}

// NewMessage creates new mail message object with default From header.
func NewMessage(to []string, subject, body string) *Message {
	return NewMessageFrom(to, setting.MailService.From, subject, body)
}
