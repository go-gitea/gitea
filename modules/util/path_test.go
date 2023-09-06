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

func TestCleanPath(t *testing.T) {
	cases := []struct {
		elems    []string
		expected string
	}{
		{[]string{}, ``},
		{[]string{``}, ``},
		{[]string{`..`}, `.`},
		{[]string{`a`}, `a`},
		{[]string{`/a/`}, `a`},
		{[]string{`../a/`, `../b`, `c/..`, `d`}, `a/b/d`},
		{[]string{`a\..\b`}, `a\..\b`},
		{[]string{`a`, ``, `b`}, `a/b`},
		{[]string{`a`, `..`, `b`}, `a/b`},
		{[]string{`lfs`, `repo/..`, `user/../path`}, `lfs/path`},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, PathJoinRel(c.elems...), "case: %v", c.elems)
	}

	cases = []struct {
		elems    []string
		expected string
	}{
		{[]string{}, ``},
		{[]string{``}, ``},
		{[]string{`..`}, `.`},
		{[]string{`a`}, `a`},
		{[]string{`/a/`}, `a`},
		{[]string{`../a/`, `../b`, `c/..`, `d`}, `a/b/d`},
		{[]string{`a\..\b`}, `b`},
		{[]string{`a`, ``, `b`}, `a/b`},
		{[]string{`a`, `..`, `b`}, `a/b`},
		{[]string{`lfs`, `repo/..`, `user/../path`}, `lfs/path`},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, PathJoinRelX(c.elems...), "case: %v", c.elems)
	}

	// for POSIX only, but the result is similar on Windows, because the first element must be an absolute path
	if isOSWindows() {
		cases = []struct {
			elems    []string
			expected string
		}{
			{[]string{`C:\..`}, `C:\`},
			{[]string{`C:\a`}, `C:\a`},
			{[]string{`C:\a/`}, `C:\a`},
			{[]string{`C:\..\a\`, `../b`, `c\..`, `d`}, `C:\a\b\d`},
			{[]string{`C:\a/..\b`}, `C:\b`},
			{[]string{`C:\a`, ``, `b`}, `C:\a\b`},
			{[]string{`C:\a`, `..`, `b`}, `C:\a\b`},
			{[]string{`C:\lfs`, `repo/..`, `user/../path`}, `C:\lfs\path`},
		}
	} else {
		cases = []struct {
			elems    []string
			expected string
		}{
			{[]string{`/..`}, `/`},
			{[]string{`/a`}, `/a`},
			{[]string{`/a/`}, `/a`},
			{[]string{`/../a/`, `../b`, `c/..`, `d`}, `/a/b/d`},
			{[]string{`/a\..\b`}, `/b`},
			{[]string{`/a`, ``, `b`}, `/a/b`},
			{[]string{`/a`, `..`, `b`}, `/a/b`},
			{[]string{`/lfs`, `repo/..`, `user/../path`}, `/lfs/path`},
		}
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, FilePathJoinAbs(c.elems[0], c.elems[1:]...), "case: %v", c.elems)
	}
}
