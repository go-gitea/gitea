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

	trueTestCasesStrict := [][]string{
		{"readme", ""},
		{"readme.md", ".md"},
		{"readme.txt", ".txt"},
	}
	falseTestCasesStrict := [][]string{
		{"readme", ".md"},
		{"readme.md", ""},
		{"readme.md", ".txt"},
		{"readme.md", "md"},
		{"readmee.md", ".md"},
		{"readme.i18n.md", ".md"},
	}

	for _, testCase := range trueTestCasesStrict {
		assert.True(t, IsReadmeFile(testCase[0], testCase[1]))
	}
	for _, testCase := range falseTestCasesStrict {
		assert.False(t, IsReadmeFile(testCase[0], testCase[1]))
	}
}
