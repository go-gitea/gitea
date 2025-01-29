// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTreeRedirectURL(t *testing.T) {
	type testcase struct {
		name                                  string
		prefix, username, reponame, remainder string
		expected                              string
	}

	cases := []testcase{
		{
			name:   "ref",
			prefix: "", username: "user2", reponame: "readme-test", remainder: "main",
			expected: "/user2/readme-test/src/main",
		},
		{
			name:   "sha",
			prefix: "", username: "user2", reponame: "readme-test", remainder: "fe495ea336f079ef2bed68648d0ba9a37cdbd4aa",
			expected: "/user2/readme-test/src/fe495ea336f079ef2bed68648d0ba9a37cdbd4aa",
		},
		{
			name:   "escape",
			prefix: "", username: "user/2%", reponame: "readme/test?", remainder: "$/&",
			expected: "/user%2F2%25/readme%2Ftest%3F/src/$/&",
		},
		{
			name:   "appPath",
			prefix: "/app-path", username: "user2", reponame: "readme-test", remainder: "main",
			expected: "/app-path/user2/readme-test/src/main",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			actual := treeRedirectURL(c.prefix, c.username, c.reponame, c.remainder)
			assert.Equal(t, c.expected, actual)
		})
	}
}
