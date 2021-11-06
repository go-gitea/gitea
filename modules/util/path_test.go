// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"net/url"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileURLToPath(t *testing.T) {
	var cases = []struct {
		url      string
		expected string
		haserror bool
		windows  bool
	}{
		// case 0
		{
			url:      "",
			haserror: true,
		},
		// case 1
		{
			url:      "http://test.io",
			haserror: true,
		},
		// case 2
		{
			url:      "file:///path",
			expected: "/path",
		},
		// case 3
		{
			url:      "file:///C:/path",
			expected: "C:/path",
			windows:  true,
		},
	}

	for n, c := range cases {
		if c.windows && runtime.GOOS != "windows" {
			continue
		}
		u, _ := url.Parse(c.url)
		p, err := FileURLToPath(u)
		if c.haserror {
			assert.Error(t, err, "case %d: should return error", n)
		} else {
			assert.NoError(t, err, "case %d: should not return error", n)
			assert.Equal(t, c.expected, p, "case %d: should be equal", n)
		}
	}
}
