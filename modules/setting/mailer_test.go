// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_loadMailerFrom(t *testing.T) {
	cases := map[string]*Mailer{
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
	for host := range cases {
		t.Run(host, func(t *testing.T) {
			cfg, _ := NewConfigProviderFromData("")
			sec := cfg.Section("mailer")
			sec.NewKey("ENABLED", "true")
			sec.NewKey("HOST", host)

			// Ensure removed settings catches the removed config setting
			assert.Len(t, checkForRemovedSettings(cfg), 1)
		})
	}
}
