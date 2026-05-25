// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func Test_loadMailerFrom(t *testing.T) {
	kases := map[string]*Mailer{
		"smtp.mydomain.test": {
			SMTPAddr: "smtp.mydomain.test",
			SMTPPort: "465",
		},
		"smtp.mydomain.test:123": {
			SMTPAddr: "smtp.mydomain.test",
			SMTPPort: "123",
		},
		":123": {
			SMTPAddr: "127.0.0.1",
			SMTPPort: "123",
		},
	}
	for host, kase := range kases {
		t.Run(host, func(t *testing.T) {
			cfg, _ := NewConfigProviderFromData("")
			sec := cfg.Section("mailer")
			sec.NewKey("ENABLED", "true")
			sec.NewKey("HOST", host)

			// Check mailer setting
			loadMailerFrom(cfg)

			assert.Equal(t, kase.SMTPAddr, MailService.SMTPAddr)
			assert.Equal(t, kase.SMTPPort, MailService.SMTPPort)
		})
	}
}

func TestLoadSettingsForInstallMailServiceFlags(t *testing.T) {
	defer test.MockVariableValue(&Service)()
	defer test.MockVariableValue(&MailService)()

	cfg, err := NewConfigProviderFromData(`
[database]
DB_TYPE = postgres

[mailer]
ENABLED = true
SMTP_ADDR = 127.0.0.1
SMTP_PORT = 465
FROM = noreply@example.com

[service]
REGISTER_EMAIL_CONFIRM = true
ENABLE_NOTIFY_MAIL = true
`)
	assert.NoError(t, err)
	loadDBSetting(cfg)
	loadServiceFrom(cfg)
	loadMailsFrom(cfg)

	assert.True(t, Service.RegisterEmailConfirm)
	assert.True(t, Service.EnableNotifyMail)
}
