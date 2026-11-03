// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanValue(t *testing.T) {
	tests := []struct {
		param  string
		expect string
	}{
		// Github behavior test cases
		{"", ""},
		{"test(0)", "test0"},
		{"test!1", "test1"},
		{"test:2", "test2"},
		{"test*3", "test3"},
		{"test！4", "test4"},
		{"test：5", "test5"},
		{"test*6", "test6"},
		{"test：6 a", "test6-a"},
		{"test：6 !b", "test6-b"},
		{"test：ad # df", "testad--df"},
		{"test：ad #23 df 2*/*", "testad-23-df-2"},
		{"test：ad 23 df 2*/*", "testad-23-df-2"},
		{"test：ad # 23 df 2*/*", "testad--23-df-2"},
		{"Anchors in Markdown", "anchors-in-markdown"},
		{"a_b_c", "a_b_c"},
		{"a-b-c", "a-b-c"},
		{"a-b-c----", "a-b-c----"},
		{"test：6a", "test6a"},
		{"test：a6", "testa6"},
		{"tes a a   a  a", "tes-a-a---a--a"},
		{"  tes a a   a  a  ", "tes-a-a---a--a"},
		{"Header with \"double quotes\"", "header-with-double-quotes"},
		{"Placeholder to force scrolling on link's click", "placeholder-to-force-scrolling-on-links-click"},
		{"tes（）", "tes"},
		{"tes（0）", "tes0"},
		{"tes{0}", "tes0"},
		{"tes[0]", "tes0"},
		{"test【0】", "test0"},
		{"tes…@a", "tesa"},
		{"tes￥& a", "tes-a"},
		{"tes= a", "tes-a"},
		{"tes|a", "tesa"},
		{"tes\\a", "tesa"},
		{"tes/a", "tesa"},
		{"a啊啊b", "a啊啊b"},
		{"c🤔️🤔️d", "cd"},
		{"a⚡a", "aa"},
		{"e.~f", "ef"},
		{"\u0939\u093f\u0928\u094d\u0926\u0940", "\u0939\u093f\u0928\u094d\u0926\u0940"},             // हिन्दी
		{"\u0e20\u0e32\u0e29\u0e32\u0e44\u0e17\u0e22", "\u0e20\u0e32\u0e29\u0e32\u0e44\u0e17\u0e22"}, // বাংলা
		{"Cafe\u0301", "cafe\u0301"},
		{"a\u20ddb", "a\u20ddb"},
	}
	for _, test := range tests {
		assert.Equal(t, []byte(test.expect), CleanValue([]byte(test.param)), test.param)
	}
}
