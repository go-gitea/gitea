// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
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
func NewMessageFrom(to []string, from, subject, htmlBody string) *Message {
	log.Trace("NewMessageFrom (htmlBody):\n%s", htmlBody)

	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to...)
	msg.SetHeader("Subject", subject)
	msg.SetDateHeader("Date", time.Now())

	body, err := html2text.FromString(htmlBody)
	if err != nil {
		log.Error(4, "html2text.FromString: %v", err)
		msg.SetBody("text/html", htmlBody)
	} else {
		msg.SetBody("text/plain", body)
		if setting.MailService.EnableHTMLAlternative {
			msg.AddAlternative("text/html", htmlBody)
		}
	}

	return &Message{
		Message: msg,
	}
}

// NewMessage creates new mail message object with default From header.
func NewMessage(to []string, subject, body string) *Message {
	return NewMessageFrom(to, setting.MailService.From, subject, body)
}
