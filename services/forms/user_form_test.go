// Copyright 2018 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"testing"

	"gitea.dev/modules/glob"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRegisterForm_IsDomainAllowed_Empty(t *testing.T) {
	defer test.MockVariableValue(&setting.Service)()
	setting.Service.EmailDomainAllowList = nil
	form := RegisterForm{}
	assert.True(t, form.IsEmailDomainAllowed())
}

func TestRegisterForm_IsDomainAllowed_InvalidEmail(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.EmailDomainAllowList, []glob.Glob{glob.MustCompile("gitea.io")})()

	for _, email := range []string{"invalid-email", "gitea.io"} {
		form := RegisterForm{Email: email}
		assert.False(t, form.IsEmailDomainAllowed())
	}
}

func TestRegisterForm_IsDomainAllowed_AllowedEmail(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.EmailDomainAllowList, []glob.Glob{glob.MustCompile("gitea.io"), glob.MustCompile("*.allow")})()

	tt := []struct {
		email string
		valid bool
	}{
		{"security@gitea.io", true},
		{"security@gITea.io", true},
		{"hack%evil.example@gitea.io", false},
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
	defer test.MockVariableValue(&setting.Service.EmailDomainBlockList, []glob.Glob{glob.MustCompile("gitea.io"), glob.MustCompile("*.block")})()

	tt := []struct {
		email string
		valid bool
	}{
		{"security@gitea.io", false},
		{"security@gitea.example", true},
		{"gitea.io!hack@gitea.example", false},
		{"invalid", true},

		{"user@my.block", false},
		{"user@my.block1", true},
	}

	for _, v := range tt {
		form := RegisterForm{Email: v.email}

		assert.Equal(t, v.valid, form.IsEmailDomainAllowed())
	}
}

func TestDetectInvalidOAuth2ApplicationRedirectURI(t *testing.T) {
	defer test.MockVariableValue(&setting.OAuth2.CustomSchemes)()
	setting.OAuth2.CustomSchemes = []string{"my-app"}
	for _, uri := range []string{"my-app:", "my-app:/foo", "http://foo", "https://foo"} {
		assert.Empty(t, DetectInvalidOAuth2ApplicationRedirectURI([]string{uri}))
	}
	for _, uri := range []string{"my-app", "ftp:", "ftp://foo", "https://[invalid"} {
		assert.Equal(t, uri, DetectInvalidOAuth2ApplicationRedirectURI([]string{uri}))
	}

	ret := DetectInvalidOAuth2ApplicationRedirectURI([]string{"my-app:", "http://foo", "https://foo"})
	assert.Empty(t, ret)
	ret = DetectInvalidOAuth2ApplicationRedirectURI([]string{"my-app:", "http://foo", "invalid", "https://foo"})
	assert.Equal(t, "invalid", ret)
}
