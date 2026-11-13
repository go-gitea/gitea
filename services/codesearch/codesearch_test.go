// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codesearch

import (
	"testing"

	"gitea.dev/modules/indexer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchQueryCapabilities(t *testing.T) {
	for _, test := range []struct {
		query string
		opts  Options
	}{
		{query: `"unterminated`},
		{query: "/word/"},
		{query: "word language:go", opts: Options{Engine: GitGrep}},
		{query: "word repo:owner/repo", opts: Options{RepoID: 1}},
		{query: "word user:owner", opts: Options{OwnerName: "owner"}},
	} {
		t.Run(test.query, func(t *testing.T) {
			result, err := Search(t.Context(), test.query, &test.opts)
			var queryError *indexer.SearchQueryError
			require.ErrorAs(t, err, &queryError)
			assert.Nil(t, result)
		})
	}
}
