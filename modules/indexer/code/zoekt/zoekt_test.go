// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build zoekt && unix

package zoekt

import (
	"testing"

	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateZoektQuery(t *testing.T) {
	b := &Indexer{}

	cases := []struct {
		name     string
		mode     indexer.SearchModeType
		keyword  string
		expected string
	}{
		// keywords are matched literally, no character of them may be swallowed or
		// interpreted by zoekt's query syntax
		{"exact", indexer.SearchModeExact, `foo`, `content_substr:"foo"`},
		{"exact with quote", indexer.SearchModeExact, `say "hi"`, `content_substr:"say \"hi\""`},
		{"exact with backslash", indexer.SearchModeExact, `a\b`, `content_substr:"a\\b"`},
		{"exact with regexp meta", indexer.SearchModeExact, `a.b*`, `content_substr:"a.b*"`},
		{"words", indexer.SearchModeWords, `foo "bar`, `(or content_substr:"foo" content_substr:"\"bar")`},

		// regexp keywords keep their escapes
		{"regexp digits", indexer.SearchModeRegexp, `\d+`, `regex:"[0-9]+"`},
		{"regexp escaped dot", indexer.SearchModeRegexp, `a\.b`, `content_substr:"a.b"`},
		{"regexp word boundary", indexer.SearchModeRegexp, `\bword\b`, `regex:"\\bword\\b"`},

		// zoekt mode passes the keyword to zoekt's own parser
		{"zoekt file", indexer.SearchModeZoekt, `file:foo`, `file_substr:"foo"`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q, err := b.generateZoektQuery(t.Context(), &internal.SearchOptions{
				Keyword:    c.keyword,
				SearchMode: c.mode,
			})
			require.NoError(t, err)
			assert.Equal(t, c.expected, q.String())
		})
	}

	t.Run("repo filter", func(t *testing.T) {
		q, err := b.generateZoektQuery(t.Context(), &internal.SearchOptions{
			Keyword:    "foo",
			SearchMode: indexer.SearchModeExact,
			RepoIDs:    []int64{1, 2},
		})
		require.NoError(t, err)
		assert.Equal(t, `(and content_substr:"foo" (repoids count:2))`, q.String())
	})

	t.Run("empty keyword", func(t *testing.T) {
		_, err := b.generateZoektQuery(t.Context(), &internal.SearchOptions{SearchMode: indexer.SearchModeExact})
		assert.Error(t, err)
	})

	t.Run("invalid regexp", func(t *testing.T) {
		_, err := b.generateZoektQuery(t.Context(), &internal.SearchOptions{
			Keyword:    `a(`,
			SearchMode: indexer.SearchModeRegexp,
		})
		assert.Error(t, err)
	})
}
