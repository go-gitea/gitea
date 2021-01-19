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

func TestPostProcess_RawHTML(t *testing.T) {
	var localMetas = map[string]string{
		"user": "go-gitea",
		"repo": "gitea",
	}
	test := func(input, expected string) {
		result, err := PostProcess([]byte(input), "https://example.com", localMetas, false)
		assert.NoError(t, err)
		assert.Equal(t, expected, string(result))
	}
	var kases = []struct {
		Input    string
		Expected string
	}{
		{
			"<A><maTH><tr><MN><bodY ÿ><temPlate></template><tH><tr></A><tH><d<bodY ",
			`<a><math><tr><mn><template></template></mn><th></th><tr></tr></tr></math></a>`,
		},
		{
			"<html><head></head><body><div></div></bodY></html>",
			`<div></div>`,
		},
	}
	for _, kase := range kases {
		test(kase.Input, kase.Expected)
	}
}
