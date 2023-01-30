// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"sort"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

func TestAmbiguousCharacters(t *testing.T) {
	for locale, ambiguous := range AmbiguousCharacters {
		assert.Equal(t, locale, ambiguous.Locale)
		assert.Equal(t, len(ambiguous.Confusable), len(ambiguous.With))
		assert.True(t, sort.SliceIsSorted(ambiguous.Confusable, func(i, j int) bool {
			return ambiguous.Confusable[i] < ambiguous.Confusable[j]
		}))

		for _, confusable := range ambiguous.Confusable {
			assert.True(t, unicode.Is(ambiguous.RangeTable, confusable))
			i := sort.Search(len(ambiguous.Confusable), func(j int) bool {
				return ambiguous.Confusable[j] >= confusable
			})
			found := i < len(ambiguous.Confusable) && ambiguous.Confusable[i] == confusable
			assert.True(t, found, "%c is not in %d", confusable, i)
		}
	}
}
