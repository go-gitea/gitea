// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeErrorCredentialURLs(t *testing.T) {
	err := errors.New("error with https://a@b.com")
	se := SanitizeErrorCredentialURLs(err)
	assert.Equal(t, "error with https://"+userPlaceholder+"@b.com", se.Error())
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
			"https://" + userPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		{
			"https://user:password@github.com/go-gitea/test_repo.git",
			"https://" + userPlaceholder + "@github.com/go-gitea/test_repo.git",
		},
		{
			"ftp://x@",
			"ftp://" + userPlaceholder + "@",
		},
		{
			"ftp://x/@",
			"ftp://x/@",
		},
		{
			"ftp://u@x/@", // test multiple @ chars
			"ftp://" + userPlaceholder + "@x/@",
		},
		{
			"😊ftp://u@x😊", // test unicode
			"😊ftp://" + userPlaceholder + "@x😊",
		},
		{
			"://@",
			"://@",
		},
		{
			"//u:p@h", // do not process URLs without explicit scheme, they are not treated as "valid" URLs because there is no scheme context in string
			"//u:p@h",
		},
		{
			"s://u@h", // the minimal pattern to be sanitized
			"s://" + userPlaceholder + "@h",
		},
		{
			"URLs in log https://u:b@h and https://u:b@h:80/, with https://h.com and u@h.com",
			"URLs in log https://" + userPlaceholder + "@h and https://" + userPlaceholder + "@h:80/, with https://h.com and u@h.com",
		},
	}

	for n, c := range cases {
		result := SanitizeCredentialURLs(c.input)
		assert.Equal(t, c.expected, result, "case %d: error should match", n)
	}
}
