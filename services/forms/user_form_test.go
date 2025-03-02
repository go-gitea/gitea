// Copyright 2018 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
)

func TestRegisterForm_IsDomainAllowed_Empty(t *testing.T) {
	oldService := setting.Service
	defer func() {
		setting.Service = oldService
	}()

	setting.Service.EmailDomainAllowList = nil

	form := RegisterForm{}

	assert.True(t, form.IsEmailDomainAllowed())
}

func TestRegisterForm_IsDomainAllowed_InvalidEmail(t *testing.T) {
	oldService := setting.Service
	defer func() {
		setting.Service = oldService
	}()

	setting.Service.EmailDomainAllowList = []glob.Glob{glob.MustCompile("gitea.io")}

	tt := []struct {
		email string
	}{
		{"invalid-email"},
		{"gitea.io"},
	}

	for _, v := range tt {
		form := RegisterForm{Email: v.email}

		assert.False(t, form.IsEmailDomainAllowed())
	}
}

func TestRegisterForm_IsDomainAllowed_AllowedEmail(t *testing.T) {
	oldService := setting.Service
	defer func() {
		setting.Service = oldService
	}()

	setting.Service.EmailDomainAllowList = []glob.Glob{glob.MustCompile("gitea.io"), glob.MustCompile("*.allow")}

	tt := []struct {
		email string
		valid bool
	}{
		{"security@gitea.io", true},
		{"security@gITea.io", true},
		{"invalid", false},
		{"seee@example.com", false},

		{"user@my.allow", true},
		{"user@my.allow1", false},
	}

	for _, v := range tt {
		form := RegisterForm{Email: v.email}

		assert.Equal(t, v.valid, form.IsEmailDomainAllowed())
	}
}

func TestRegisterForm_IsDomainAllowed_BlockedEmail(t *testing.T) {
	oldService := setting.Service
	defer func() {
		setting.Service = oldService
	}()

	setting.Service.EmailDomainAllowList = nil
	setting.Service.EmailDomainBlockList = []glob.Glob{glob.MustCompile("gitea.io"), glob.MustCompile("*.block")}

	tt := []struct {
		email string
		valid bool
	}{
		{"security@gitea.io", false},
		{"security@gitea.example", true},
		{"invalid", true},

		{"user@my.block", false},
		{"user@my.block1", true},
	}

	for _, v := range tt {
		form := RegisterForm{Email: v.email}

		assert.Equal(t, v.valid, form.IsEmailDomainAllowed())
	}
}
