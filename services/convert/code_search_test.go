// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	code_indexer "gitea.dev/modules/indexer/code"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestToCodeSearchTextMatch(t *testing.T) {
	textMatch := toCodeSearchTextMatch("url", "héllo wörld", []code_indexer.MatchRange{{Start: 0, End: 6}, {Start: 7, End: 13}})
	assert.Equal(t, []*api.CodeSearchTextMatchTerm{
		{Text: "héllo", Indices: []int{0, 5}},
		{Text: "wörld", Indices: []int{6, 11}},
	}, textMatch.Matches)

	assert.Empty(t, toCodeSearchTextMatch("url", "abc", nil).Matches)
}
