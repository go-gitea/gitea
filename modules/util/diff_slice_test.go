// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffSliceBasic(t *testing.T) {
	for _, testCase := range []struct {
		name                           string
		oldSlice, newSlice             []int
		expectedAdded, expectedRemoved []int
	}{
		// Typical integer cases
		{"additions", []int{1, 2}, []int{1, 2, 3}, []int{3}, nil},
		{"removals", []int{1, 2, 3}, []int{1, 2}, nil, []int{3}},
		{"no changes", []int{1, 2}, []int{1, 2}, nil, nil},
		{"empty slices", []int{}, []int{}, nil, nil},
		{"overlapping elements", []int{1, 2, 4}, []int{2, 3, 4}, []int{3}, []int{1}},
		{"order and duplicates", []int{1, 2, 2, 3}, []int{2, 4, 2, 5}, []int{4, 5}, []int{1, 3}},
		// Test case from issue: newSlice contains [4, 4, 5] and oldSlice is [1]
		// added should return [4, 5], not [4, 4, 5]
		{"deduplicates added", []int{1}, []int{4, 4, 5}, []int{4, 5}, []int{1}},
		{"deduplicates removed", []int{1, 1, 2}, []int{3}, []int{3}, []int{1, 2}},
		{"deduplicates both", []int{1, 1, 2, 2}, []int{3, 3, 4, 4}, []int{3, 4}, []int{1, 2}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			added, removed := DiffSlice(testCase.oldSlice, testCase.newSlice)
			if testCase.expectedAdded == nil {
				assert.Empty(t, added)
			} else {
				assert.Equal(t, testCase.expectedAdded, added)
			}
			if testCase.expectedRemoved == nil {
				assert.Empty(t, removed)
			} else {
				assert.Equal(t, testCase.expectedRemoved, removed)
			}
		})
	}
}
