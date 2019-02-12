// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"code.gitea.io/gitea/modules/setting"
)

func Test_IsValidURL(t *testing.T) {
	cases := []struct {
		description string
		url         string
		valid       bool
	}{
		{
			description: "Empty URL",
			url:         "",
			valid:       false,
		},
		{
			description: "Loobpack IPv4 URL",
			url:         "http://127.0.1.1:5678/",
			valid:       true,
		},
		{
			description: "Loobpack IPv6 URL",
			url:         "https://[::1]/",
			valid:       true,
		},
		{
			description: "Missing semicolon after schema",
			url:         "http//meh/",
			valid:       false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			assert.Equal(t, testCase.valid, IsValidURL(testCase.url))
		})
	}
}

func Test_IsValidExternalURL(t *testing.T) {
	setting.AppURL = "https://try.gitea.io/"

	cases := []struct {
		description string
		url         string
		valid       bool
	}{
		{
			description: "Current instance URL",
			url:         "https://try.gitea.io/test",
			valid:       true,
		},
		{
			description: "Loobpack IPv4 URL",
			url:         "http://127.0.1.1:5678/",
			valid:       false,
		},
		{
			description: "Current instance API URL",
			url:         "https://try.gitea.io/api/v1/user/follow",
			valid:       false,
		},
		{
			description: "Local network URL",
			url:         "http://192.168.1.2/api/v1/user/follow",
			valid:       true,
		},
		{
			description: "Local URL",
			url:         "http://LOCALHOST:1234/whatever",
			valid:       false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			assert.Equal(t, testCase.valid, IsValidExternalURL(testCase.url))
		})
	}
}
