// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2015 Kenneth Shaw
// SPDX-License-Identifier: MIT

package emoji

import (
	"testing"

	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {
	a := FromCode("\U0001f37a")
	b := FromCode("🍺")
	c := FromAlias(":beer:")
	d := FromAlias("beer")

	assert.Equal(t, a, b)
	assert.Equal(t, b, c)
	assert.Equal(t, c, d)

	m := FromCode("\U0001f44d")
	n := FromAlias(":thumbsup:")
	o := FromAlias("+1")

	assert.Equal(t, m, n)
	assert.Equal(t, m, o)

	defer test.MockVariableValue(&setting.UI.EnabledEmojisSet, container.SetOf("thumbsup"))()
	defer globalVarsStore.Store(nil)
	globalVarsStore.Store(nil)
	a = FromCode("\U0001f37a")
	c = FromAlias(":beer:")
	m = FromCode("\U0001f44d")
	n = FromAlias(":thumbsup:")
	o = FromAlias("+1")
	assert.Nil(t, a)
	assert.Nil(t, c)
	assert.NotNil(t, m)
	assert.NotNil(t, n)
	assert.Nil(t, o)
}

func TestReplacers(t *testing.T) {
	tests := []struct {
		f      func(string) string
		v, exp string
	}{
		{ReplaceCodes, ":thumbsup: +1 for \U0001f37a! 🍺 \U0001f44d", ":thumbsup: +1 for :beer:! :beer: :+1:"},
		{ReplaceAliases, ":thumbsup: +1 :+1: :beer:", "\U0001f44d +1 \U0001f44d \U0001f37a"},
	}

	for i, x := range tests {
		s := x.f(x.v)
		assert.Equalf(t, x.exp, s, "test %d `%s` expected `%s`, got: `%s`", i, x.v, x.exp, s)
	}
}

const (
	testInputWithEmojis = "This is a test string containing some emojis like \U0001f44d and \U0001f37a and some text in between."
	testInputNoEmojis   = "This is a test string containing no emojis at all, just plain old ASCII text, which should ideally be scanned very quickly by our trie implementation."
)

func TestFindEmojiSubmatchIndex(t *testing.T) {
	type testcase struct {
		input    string
		expected []int
	}

	testCases := []testcase{
		{
			"\U0001f44d",
			[]int{0, len("\U0001f44d")},
		},
		{
			"\U0001f44d +1 \U0001f44d \U0001f37a",
			[]int{0, 4},
		},
		{
			" \U0001f44d",
			[]int{1, 1 + len("\U0001f44d")},
		},
		{
			"\u0001\U0001f44d",
			[]int{1, 1 + len("\U0001f44d")},
		},
		{
			// This package can handle keycap emoji if it is registered in the emoji data.
			// However, many other places (e.g.: markup rendering) also might not handle such cases correctly.
			// For example: how is "**{U+FE0F}{U+20E3}**" rendered in Markdown/Markup?
			"a 8\U0000fe0f\U000020e3 b", // keycap emoji "8\ufe0f\u20e3" in emoji data
			[]int{2, 2 + len("8\U0000fe0f\U000020e3")},
		},
		{
			testInputWithEmojis,
			[]int{50, 54},
		},
		{
			testInputNoEmojis,
			nil,
		},
	}

	for _, tc := range testCases {
		actual := FindEmojiSubmatchIndex(tc.input)
		assert.Equal(t, tc.expected, actual)
	}
}

func BenchmarkFindEmojiSubmatchIndex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FindEmojiSubmatchIndex(testInputWithEmojis)
	}
}

func BenchmarkFindEmojiSubmatchIndexNoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FindEmojiSubmatchIndex(testInputNoEmojis)
	}
}
