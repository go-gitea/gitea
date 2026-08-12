// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
)

func Test_loadMailerFrom(t *testing.T) {
	defer test.MockVariableValue(&MailService)()

	smtp := func(addr, port string) []configCheck {
		return []configCheck{
			fieldOf("HOST", func() string { return MailService.SMTPAddr }, addr),
			fieldOf("HOST", func() string { return MailService.SMTPPort }, port),
		}
	}

	testConfigLoad(t, []any{loadMailerFrom}, []configTestCase{
		{
			name: "host without a port",
			ini:  "[mailer]\nENABLED = true\nHOST = smtp.mydomain.test",
			want: smtp("smtp.mydomain.test", "465"),
		},
		{
			name: "host with a port",
			ini:  "[mailer]\nENABLED = true\nHOST = smtp.mydomain.test:123",
			want: smtp("smtp.mydomain.test", "123"),
		},
		{
			name: "port only",
			ini:  "[mailer]\nENABLED = true\nHOST = :123",
			want: smtp("127.0.0.1", "123"),
		},
	})
}

func TestLoadSettingsForInstallMailServiceFlags(t *testing.T) {
	defer test.MockVariableValue(&Service)()
	defer test.MockVariableValue(&MailService)()

	testConfigLoad(t, []any{loadDBSetting, loadServiceFrom, loadMailsFrom}, []configTestCase{
		{
			name: "a configured mailer enables the mail services",
			ini: `
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
`,
			want: []configCheck{
				field("REGISTER_EMAIL_CONFIRM", &Service.RegisterEmailConfirm, true),
				field("ENABLE_NOTIFY_MAIL", &Service.EnableNotifyMail, true),
			},
		},
	})
}
