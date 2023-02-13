// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"net/url"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileURLToPath(t *testing.T) {
	cases := []struct {
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

func TestMisc_IsReadmeFileName(t *testing.T) {
	trueTestCases := []string{
		"readme",
		"README",
		"readME.mdown",
		"README.md",
		"readme.i18n.md",
	}
	falseTestCases := []string{
		"test.md",
		"wow.MARKDOWN",
		"LOL.mDoWn",
		"test",
		"abcdefg",
		"abcdefghijklmnopqrstuvwxyz",
		"test.md.test",
		"readmf",
	}

	for _, testCase := range trueTestCases {
		assert.True(t, IsReadmeFileName(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, IsReadmeFileName(testCase))
	}

	type extensionTestcase struct {
		name     string
		expected bool
		idx      int
	}

	exts := []string{".md", ".txt", ""}
	testCasesExtensions := []extensionTestcase{
		{
			name:     "readme",
			expected: true,
			idx:      2,
		},
		{
			name:     "readme.md",
			expected: true,
			idx:      0,
		},
		{
			name:     "README.md",
			expected: true,
			idx:      0,
		},
		{
			name:     "ReAdMe.Md",
			expected: true,
			idx:      0,
		},
		{
			name:     "readme.txt",
			expected: true,
			idx:      1,
		},
		{
			name:     "readme.doc",
			expected: true,
			idx:      3,
		},
		{
			name: "readmee.md",
		},
		{
			name:     "readme..",
			expected: true,
			idx:      3,
		},
	}

	for _, testCase := range testCasesExtensions {
		idx, ok := IsReadmeFileExtension(testCase.name, exts...)
		assert.Equal(t, testCase.expected, ok)
		assert.Equal(t, testCase.idx, idx)
	}
}
