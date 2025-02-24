// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net"
	"strings"
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
			SMTPAddr: "",
			SMTPPort: "123",
		},
	}
	for host, kase := range kases {
		t.Run(host, func(t *testing.T) {
			cfg, _ := NewConfigProviderFromData("")
			sec := cfg.Section("mailer")
			sec.NewKey("ENABLED", "true")
			if strings.Contains(host, ":") {
				addr, port, err := net.SplitHostPort(host)
				assert.NoError(t, err)
				sec.NewKey("SMTP_ADDR", addr)
				sec.NewKey("SMTP_PORT", port)
			} else {
				sec.NewKey("SMTP_ADDR", host)
			}

			// Check mailer setting
			loadMailerFrom(cfg)

			assert.EqualValues(t, kase.SMTPAddr, MailService.SMTPAddr)
			assert.EqualValues(t, kase.SMTPPort, MailService.SMTPPort)
		})
	}
}
