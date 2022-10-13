// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup_test

import (
	"testing"

	. "code.gitea.io/gitea/modules/markup"

	_ "code.gitea.io/gitea/modules/markup/markdown"

	"github.com/stretchr/testify/assert"
)

func TestMisc_IsReadmeFile(t *testing.T) {
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
		assert.True(t, IsReadmeFile(testCase))
	}
	for _, testCase := range falseTestCases {
		assert.False(t, IsReadmeFile(testCase))
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
