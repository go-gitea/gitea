// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"testing"

	"gitea.dev/modules/indexer/code/internal"

	"github.com/stretchr/testify/assert"
)

func TestHighlightMatchRanges(t *testing.T) {
	assert.Nil(t, highlightMatchRanges(nil))
	assert.Nil(t, highlightMatchRanges([]string{"no match"}))
	assert.Equal(t, []internal.MatchRange{{Start: 11, End: 16}, {Start: 21, End: 24}},
		highlightMatchRanges([]string{"test index <em>start</em> and <em>end</em>"}))
	assert.Equal(t, []internal.MatchRange{{Start: 0, End: 3}}, highlightMatchRanges([]string{"<em>abc</em><em>unclosed"}))
}
