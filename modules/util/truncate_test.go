// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEllipsisString(t *testing.T) {
	cases := []struct {
		limit int

		input, left, right string
	}{
		{limit: 0, input: "abcde", left: "", right: "…abcde"},
		{limit: 1, input: "abcde", left: "", right: "…abcde"},
		{limit: 2, input: "abcde", left: "", right: "…abcde"},
		{limit: 3, input: "abcde", left: "…", right: "…abcde"},
		{limit: 4, input: "abcde", left: "a…", right: "…bcde"},
		{limit: 5, input: "abcde", left: "abcde", right: ""},
		{limit: 6, input: "abcde", left: "abcde", right: ""},
		{limit: 7, input: "abcde", left: "abcde", right: ""},

		// a CJK char or emoji is considered as 2-ASCII width, the ellipsis is 3-ASCII width
		{limit: 0, input: "测试文本", left: "", right: "…测试文本"},
		{limit: 1, input: "测试文本", left: "", right: "…测试文本"},
		{limit: 2, input: "测试文本", left: "", right: "…测试文本"},
		{limit: 3, input: "测试文本", left: "…", right: "…测试文本"},
		{limit: 4, input: "测试文本", left: "…", right: "…测试文本"},
		{limit: 5, input: "测试文本", left: "测…", right: "…试文本"},
		{limit: 6, input: "测试文本", left: "测…", right: "…试文本"},
		{limit: 7, input: "测试文本", left: "测试…", right: "…文本"},
		{limit: 8, input: "测试文本", left: "测试文本", right: ""},
		{limit: 9, input: "测试文本", left: "测试文本", right: ""},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s(%d)", c.input, c.limit), func(t *testing.T) {
			left, right := EllipsisDisplayStringX(c.input, c.limit)
			assert.Equal(t, c.left, left, "left")
			assert.Equal(t, c.right, right, "right")
		})
	}

	t.Run("LongInput", func(t *testing.T) {
		left, right := EllipsisDisplayStringX(strings.Repeat("abc", 240), 90)
		assert.Equal(t, strings.Repeat("abc", 29)+"…", left)
		assert.Equal(t, "…"+strings.Repeat("abc", 211), right)
	})

	t.Run("InvalidUtf8", func(t *testing.T) {
		invalidCases := []struct {
			limit       int
			left, right string
		}{
			{limit: 0, left: "", right: "...\xef\x03\xfe\xef\x03\xfe"},
			{limit: 1, left: "", right: "...\xef\x03\xfe\xef\x03\xfe"},
			{limit: 2, left: "", right: "...\xef\x03\xfe\xef\x03\xfe"},
			{limit: 3, left: "...", right: "...\xef\x03\xfe\xef\x03\xfe"},
			{limit: 4, left: "...", right: "...\xef\x03\xfe\xef\x03\xfe"},
			{limit: 5, left: "\xef\x03\xfe...", right: "...\xef\x03\xfe"},
			{limit: 6, left: "\xef\x03\xfe\xef\x03\xfe", right: ""},
			{limit: 7, left: "\xef\x03\xfe\xef\x03\xfe", right: ""},
		}
		for _, c := range invalidCases {
			t.Run(fmt.Sprintf("%d", c.limit), func(t *testing.T) {
				left, right := EllipsisDisplayStringX("\xef\x03\xfe\xef\x03\xfe", c.limit)
				assert.Equal(t, c.left, left, "left")
				assert.Equal(t, c.right, right, "right")
			})
		}
	})

	t.Run("IsLikelyEllipsisLeftPart", func(t *testing.T) {
		assert.True(t, IsLikelyEllipsisLeftPart("abcde…"))
		assert.True(t, IsLikelyEllipsisLeftPart("abcde..."))
	})
}

func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "", TruncateRunes("", 0))
	assert.Equal(t, "", TruncateRunes("", 1))

	assert.Equal(t, "", TruncateRunes("ab", 0))
	assert.Equal(t, "a", TruncateRunes("ab", 1))
	assert.Equal(t, "ab", TruncateRunes("ab", 2))
	assert.Equal(t, "ab", TruncateRunes("ab", 3))

	assert.Equal(t, "", TruncateRunes("测试", 0))
	assert.Equal(t, "测", TruncateRunes("测试", 1))
	assert.Equal(t, "测试", TruncateRunes("测试", 2))
	assert.Equal(t, "测试", TruncateRunes("测试", 3))
}
