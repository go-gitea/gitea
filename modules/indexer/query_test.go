// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSearchQuery(t *testing.T) {
	cases := []struct {
		q        string
		expected SearchQuery
	}{
		{"", SearchQuery{}},
		{"foo  bar", SearchQuery{Keyword: "foo bar"}},
		{"std::string a:b", SearchQuery{Keyword: "std::string a:b"}},
		{"foo path:src/x -path:vendor", SearchQuery{Keyword: "foo", Qualifiers: []SearchQualifier{
			{Name: "path", Value: "src/x"},
			{Name: "path", Value: "vendor", Exclude: true},
		}}},
		{`path:"my dir" "foo bar"`, SearchQuery{Keyword: "foo bar", Qualifiers: []SearchQualifier{{Name: "path", Value: "my dir"}}}},
		{`"path:src" path: x`, SearchQuery{Keyword: "path:src path: x"}},
		{`say \"hi\" ""`, SearchQuery{Keyword: `say "hi"`}},
		{`"unclosed quote`, SearchQuery{Keyword: "unclosed quote"}},
	}
	for _, c := range cases {
		assert.Equal(t, &c.expected, ParseSearchQuery(c.q, "path"), "q=%q", c.q)
	}

	query := ParseSearchQuery("repo:a/b -repo:c/d repo:e/f x", "repo")
	assert.Equal(t, []string{"a/b", "e/f"}, query.Values("repo"))
	assert.Nil(t, query.Values("path"))
}
