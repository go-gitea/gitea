// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_loadMailerFrom(t *testing.T) {
	kases := map[string]*Mailer{
		"smtp.mydomain.com": {
			SMTPAddr: "smtp.mydomain.com",
			SMTPPort: "465",
		},
		"smtp.mydomain.com:123": {
			SMTPAddr: "smtp.mydomain.com",
			SMTPPort: "123",
		},
		":123": {
			SMTPAddr: "127.0.0.1",
			SMTPPort: "123",
		},
	}
	for host, kase := range kases {
		t.Run(host, func(t *testing.T) {
			cfg, _ := NewConfigProviderFromData(fmt.Sprintf(`
[mailer]
ENABLED = true
SMTP_ADDR = %s
SMTP_PORT = %s
`, kase.SMTPAddr, kase.SMTPPort))
			// Check mailer setting
			loadMailerFrom(cfg)

			assert.EqualValues(t, kase.SMTPAddr, MailService.SMTPAddr)
			assert.EqualValues(t, kase.SMTPPort, MailService.SMTPPort)
		})
	}
}
