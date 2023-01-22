// Copyright 2021 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGenerateMessageID(t *testing.T) {
	mailService := setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	date := time.Date(2000, 1, 2, 3, 4, 5, 6, time.UTC)
	m := NewMessageFrom(nil, "display-name", "from-address", "subject", "body")
	m.Date = date
	gm := m.ToMessage()
	assert.Equal(t, "<autogen-946782245000-41e8fc54a8ad3a3f@localhost>", gm.GetHeader("Message-ID")[0])

	m = NewMessageFrom([]string{"a@b.com"}, "display-name", "from-address", "subject", "body")
	m.Date = date
	gm = m.ToMessage()
	assert.Equal(t, "<autogen-946782245000-cc88ce3cfe9bd04f@localhost>", gm.GetHeader("Message-ID")[0])

	m = NewMessageFrom([]string{"a@b.com"}, "display-name", "from-address", "subject", "body")
	m.SetHeader("Message-ID", "<msg-d@domain.com>")
	gm = m.ToMessage()
	assert.Equal(t, "<msg-d@domain.com>", gm.GetHeader("Message-ID")[0])
}
