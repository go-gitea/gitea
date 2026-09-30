// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeErrorCredentialURLs(t *testing.T) {
	err := errors.New("error with https://a@b.com")
	se := SanitizeErrorCredentialURLs(err)
	assert.Equal(t, "error with https://"+userInfoPlaceholder+"@b.com", se.Error())
}

func TestSanitizeCredentialURLs(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			"https://github.com/go-gitea/test_repo.git",
			"https://github.com/go-gitea/test_repo.git",
		},
		{
			"https://mytoken@github.com/go-gitea/test_repo.git",
			"https://" + userInfoPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		{
			"https://user:password@github.com/go-gitea/test_repo.git",
			"https://" + userInfoPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		{
			"https://user:password@[::]/go-gitea/test_repo.git",
			"https://" + userInfoPlaceholder + "@[::]/go-gitea/test_repo.git",
		},
		{
			"https://user:password@[2001:db8::1]:8080/go-gitea/test_repo.git",
			"https://" + userInfoPlaceholder + "@[2001:db8::1]:8080/go-gitea/test_repo.git",
		},
		{
			"see https://u:p@[::1]/x and https://u2:p2@h2",
			"see https://" + userInfoPlaceholder + "@[::1]/x and https://" + userInfoPlaceholder + "@h2",
		},
		{
			"https://user:secret@[unclosed-ipv6",
			"https://user:secret@[unclosed-ipv6",
		},
		{
			"https://user:secret@[invalid-ipv6]",
			"https://user:secret@[invalid-ipv6]",
		},
		{
			"ftp://x@",
			"ftp://x@",
		},
		{
			"ftp://x/@",
			"ftp://x/@",
		},
		{
			"ftp://u@x/@", // test multiple @ chars
			"ftp://" + userInfoPlaceholder + "@x/@",
		},
		{
			"😊ftp://u@x😊", // test unicode
			"😊ftp://" + userInfoPlaceholder + "@x😊",
		},
		{
			"://@",
			"://@",
		},
		{
			"//u:p@h",
			"//" + userInfoPlaceholder + "@h",
		},
		{
			"s://u@h",
			"s://" + userInfoPlaceholder + "@h",
		},
		{
			"URLs in log https://u:b@h and https://u:b@h:80/, with https://h.com and u@h.com",
			"URLs in log https://" + userInfoPlaceholder + "@h and https://" + userInfoPlaceholder + "@h:80/, with https://h.com and u@h.com",
		},
		{
			"fatal: unable to look up username:token@github.com (port 9418)",
			"fatal: unable to look up " + userInfoPlaceholder + "@github.com (port 9418)",
		},
		{
			"git failed for user:token@github.com/go-gitea/test_repo.git",
			"git failed for " + userInfoPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		{
			// SSH-form git URL ("git@host:path") must not let a later credential URL through
			"failed remote git@github.com:foo, retried via https://user:tok@github.com/foo",
			"failed remote git@github.com:foo, retried via https://" + userInfoPlaceholder + "@github.com/foo",
		},
	}

	for n, c := range cases {
		result := SanitizeCredentialURLs(c.input)
		assert.Equal(t, c.expected, result, "case %d: error should match", n)
	}
}
