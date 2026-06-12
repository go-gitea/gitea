// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func str2url(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

func TestDetermineEndpoint(t *testing.T) {
	// Test cases
	cases := []struct {
		cloneurl string
		lfsurl   string
		expected *url.URL
	}{
		// case 0
		{
			cloneurl: "",
			lfsurl:   "",
			expected: nil,
		},
		// case 1
		{
			cloneurl: "https://git.com/repo",
			lfsurl:   "",
			expected: str2url("https://git.com/repo.git/info/lfs"),
		},
		// case 2
		{
			cloneurl: "https://git.com/repo.git",
			lfsurl:   "",
			expected: str2url("https://git.com/repo.git/info/lfs"),
		},
		// case 3
		{
			cloneurl: "",
			lfsurl:   "https://gitlfs.com/repo",
			expected: str2url("https://gitlfs.com/repo"),
		},
		// case 4
		{
			cloneurl: "https://git.com/repo.git",
			lfsurl:   "https://gitlfs.com/repo",
			expected: str2url("https://gitlfs.com/repo"),
		},
		// case 5
		{
			cloneurl: "git://git.com/repo.git",
			lfsurl:   "",
			expected: str2url("https://git.com/repo.git/info/lfs"),
		},
		// case 6
		{
			cloneurl: "",
			lfsurl:   "git://gitlfs.com/repo",
			expected: str2url("https://gitlfs.com/repo"),
		},
		// case 7: ssh remotes have no LFS endpoint, callers must handle nil (#38016)
		{
			cloneurl: "ssh://git@example.com/owner/repo.git",
			lfsurl:   "",
			expected: nil,
		},
		// case 8: scp-like ssh syntax
		{
			cloneurl: "git@example.com:owner/repo.git",
			lfsurl:   "",
			expected: nil,
		},
	}

	for n, c := range cases {
		ep := DetermineEndpoint(c.cloneurl, c.lfsurl)

		assert.Equal(t, c.expected, ep, "case %d: error should match", n)
	}
}
