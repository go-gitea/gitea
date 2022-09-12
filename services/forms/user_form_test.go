// Copyright 2018 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forms

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
)

// BuildEmailGlobs takes in an array of strings and
// builds an array of compiled globs used to do
// pattern matching in IsEmailDomainAllowed. A compiled list
func BuildEmailGlobs(list []string) []glob.Glob {
	var EmailList []glob.Glob

	for _, s := range list {
		if g, err := glob.Compile(s); err == nil {
			EmailList = append(EmailList, g)
		}
	}

	return EmailList
}

func TestRegisterForm_IsDomainAllowed_Empty(t *testing.T) {
	_ = setting.Service

	setting.Service.EmailDomainWhitelist = BuildEmailGlobs([]string{})

	form := RegisterForm{}

	assert.True(t, form.IsEmailDomainAllowed())
}

func TestRegisterForm_IsDomainAllowed_InvalidEmail(t *testing.T) {
	_ = setting.Service

	setting.Service.EmailDomainWhitelist = BuildEmailGlobs([]string{"gitea.io"})

	tt := []struct {
		email string
	}{
		{"securitygieqqq"},
		{"hdudhdd"},
	}

	for _, v := range tt {
		form := RegisterForm{Email: v.email}

		assert.False(t, form.IsEmailDomainAllowed())
	}
}

func TestRegisterForm_IsDomainAllowed_WhitelistedEmail(t *testing.T) {
	_ = setting.Service

	setting.Service.EmailDomainWhitelist = BuildEmailGlobs([]string{"gitea.io", "*.gov"})

	tt := []struct {
		email string
		valid bool
	}{
		{"security@gitea.io", true},
		{"security@gITea.io", true},
		{"hdudhdd", false},
		{"seee@example.com", false},
		{"security@fishsauce.gov", true},
	}

	for _, v := range tt {
		form := RegisterForm{Email: v.email}

		assert.Equal(t, v.valid, form.IsEmailDomainAllowed())
	}
}

func TestRegisterForm_IsDomainAllowed_BlocklistedEmail(t *testing.T) {
	_ = setting.Service

	setting.Service.EmailDomainWhitelist = BuildEmailGlobs([]string{})
	setting.Service.EmailDomainBlocklist = BuildEmailGlobs([]string{"gitea.io", "*.gov"})

	tt := []struct {
		email string
		valid bool
	}{
		{"security@gitea.io", false},
		{"security@gitea.example", true},
		{"hdudhdd", true},
		{"security@fishsauce.gov", false},
	}

	for _, v := range tt {
		form := RegisterForm{Email: v.email}

		assert.Equal(t, v.valid, form.IsEmailDomainAllowed())
	}
}
