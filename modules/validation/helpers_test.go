// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
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
			description: "Loopback IPv4 URL",
			url:         "http://127.0.1.1:5678/",
			valid:       true,
		},
		{
			description: "Loopback IPv6 URL",
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
	defer test.MockVariableValue(&setting.AppURL, "https://try.gitea.io/")()

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
			description: "Loopback IPv4 URL",
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

func Test_IsValidExternalTrackerURLFormat(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://try.gitea.io/")()

	cases := []struct {
		description string
		url         string
		valid       bool
	}{
		{
			description: "Correct external tracker URL with all placeholders",
			url:         "https://github.com/{user}/{repo}/issues/{index}",
			valid:       true,
		},
		{
			description: "Local external tracker URL with all placeholders",
			url:         "https://127.0.0.1/{user}/{repo}/issues/{index}",
			valid:       false,
		},
		{
			description: "External tracker URL with typo placeholder",
			url:         "https://github.com/{user}/{repo/issues/{index}",
			valid:       false,
		},
		{
			description: "External tracker URL with typo placeholder",
			url:         "https://github.com/[user}/{repo/issues/{index}",
			valid:       false,
		},
		{
			description: "External tracker URL with typo placeholder",
			url:         "https://github.com/{user}/repo}/issues/{index}",
			valid:       false,
		},
		{
			description: "External tracker URL missing optional placeholder",
			url:         "https://github.com/{user}/issues/{index}",
			valid:       true,
		},
		{
			description: "External tracker URL missing optional placeholder",
			url:         "https://github.com/{repo}/issues/{index}",
			valid:       true,
		},
		{
			description: "External tracker URL missing optional placeholder",
			url:         "https://github.com/issues/{index}",
			valid:       true,
		},
		{
			description: "External tracker URL missing optional placeholder",
			url:         "https://github.com/issues/{user}",
			valid:       true,
		},
		{
			description: "External tracker URL with similar placeholder names test",
			url:         "https://github.com/user/repo/issues/{index}",
			valid:       true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			assert.Equal(t, testCase.valid, IsValidExternalTrackerURLFormat(testCase.url))
		})
	}
}

func TestIsValidUsername(t *testing.T) {
	tests := []struct {
		arg  string
		want bool
	}{
		{arg: "a", want: true},
		{arg: "abc", want: true},
		{arg: "0.b-c", want: true},
		{arg: "a.b-c_d", want: true},
		{arg: "", want: false},
		{arg: ".abc", want: false},
		{arg: "abc.", want: false},
		{arg: "a..bc", want: false},
		{arg: "a...bc", want: false},
		{arg: "a.-bc", want: false},
		{arg: "a._bc", want: false},
		{arg: "a_-bc", want: false},
		{arg: "a/bc", want: false},
		{arg: "☁️", want: false},
		{arg: "-", want: false},
		{arg: "--diff", want: false},
		{arg: "-im-here", want: false},
		{arg: "a space", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			assert.Equalf(t, tt.want, IsValidUsername(tt.arg), "IsValidUsername(%v)", tt.arg)
		})
	}
}
