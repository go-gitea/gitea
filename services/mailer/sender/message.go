// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/jaytaylor/html2text"
	"gopkg.in/gomail.v2"
)

// Message mail body and log info
type Message struct {
	Info            string // Message information for log purpose.
	FromAddress     string
	FromDisplayName string
	To              string // Use only one recipient to prevent leaking of addresses
	ReplyTo         string
	Subject         string
	Date            time.Time
	Body            string
	Headers         map[string][]string
}

// ToMessage converts a Message to gomail.Message
func (m *Message) ToMessage() *gomail.Message {
	msg := gomail.NewMessage()
	msg.SetAddressHeader("From", m.FromAddress, m.FromDisplayName)
	msg.SetHeader("To", m.To)
	if m.ReplyTo != "" {
		msg.SetHeader("Reply-To", m.ReplyTo)
	}
	for header := range m.Headers {
		msg.SetHeader(header, m.Headers[header]...)
	}

	if setting.MailService.SubjectPrefix != "" {
		msg.SetHeader("Subject", setting.MailService.SubjectPrefix+" "+m.Subject)
	} else {
		msg.SetHeader("Subject", m.Subject)
	}
	msg.SetDateHeader("Date", m.Date)
	msg.SetHeader("X-Auto-Response-Suppress", "All")

	plainBody, err := html2text.FromString(m.Body)
	if err != nil || setting.MailService.SendAsPlainText {
		if strings.Contains(base.TruncateString(m.Body, 100), "<html>") {
			log.Warn("Mail contains HTML but configured to send as plain text.")
		}
		msg.SetBody("text/plain", plainBody)
	} else {
		msg.SetBody("text/plain", plainBody)
		msg.AddAlternative("text/html", m.Body)
	}

	if len(msg.GetHeader("Message-ID")) == 0 {
		msg.SetHeader("Message-ID", m.generateAutoMessageID())
	}

	for k, v := range setting.MailService.OverrideHeader {
		if len(msg.GetHeader(k)) != 0 {
			log.Debug("Mailer override header '%s' as per config", k)
		}
		msg.SetHeader(k, v...)
	}

	return msg
}

// SetHeader adds additional headers to a message
func (m *Message) SetHeader(field string, value ...string) {
	m.Headers[field] = value
}

func (m *Message) generateAutoMessageID() string {
	dateMs := m.Date.UnixNano() / 1e6
	h := fnv.New64()
	if len(m.To) > 0 {
		_, _ = h.Write([]byte(m.To))
	}
	_, _ = h.Write([]byte(m.Subject))
	_, _ = h.Write([]byte(m.Body))
	return fmt.Sprintf("<autogen-%d-%016x@%s>", dateMs, h.Sum64(), setting.Domain)
}

// NewMessageFrom creates new mail message object with custom From header.
func NewMessageFrom(to, fromDisplayName, fromAddress, subject, body string) *Message {
	log.Trace("NewMessageFrom (body):\n%s", body)

	return &Message{
		FromAddress:     fromAddress,
		FromDisplayName: fromDisplayName,
		To:              to,
		Subject:         subject,
		Date:            time.Now(),
		Body:            body,
		Headers:         map[string][]string{},
	}
}

// NewMessage creates new mail message object with default From header.
func NewMessage(to, subject, body string) *Message {
	return NewMessageFrom(to, setting.MailService.FromName, setting.MailService.FromEmail, subject, body)
}
