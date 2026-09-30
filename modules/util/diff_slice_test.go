// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffSliceBasic(t *testing.T) {
	// Typical integer cases
	t.Run("additions", func(t *testing.T) {
		added, removed := DiffSlice([]int{1, 2}, []int{1, 2, 3})
		assert.Equal(t, []int{3}, added)
		assert.Empty(t, removed)
	})

	t.Run("removals", func(t *testing.T) {
		added, removed := DiffSlice([]int{1, 2, 3}, []int{1, 2})
		assert.Empty(t, added)
		assert.Equal(t, []int{3}, removed)
	})

	t.Run("no changes", func(t *testing.T) {
		added, removed := DiffSlice([]int{1, 2}, []int{1, 2})
		assert.Empty(t, added)
		assert.Empty(t, removed)
	})

	t.Run("empty slices", func(t *testing.T) {
		added, removed := DiffSlice([]int{}, []int{})
		assert.Empty(t, added)
		assert.Empty(t, removed)
	})

	t.Run("overlapping elements", func(t *testing.T) {
		added, removed := DiffSlice([]int{1, 2, 4}, []int{2, 3, 4})
		assert.Equal(t, []int{3}, added)
		assert.Equal(t, []int{1}, removed)
	})
}

func TestDiffSliceOrderAndDuplicates(t *testing.T) {
	oldSlice := []int{1, 2, 2, 3}
	newSlice := []int{2, 4, 2, 5}

	added, removed := DiffSlice(oldSlice, newSlice)
	assert.Equal(t, []int{4, 5}, added)
	assert.Equal(t, []int{1, 3}, removed)
}

func TestDiffSliceDeduplicatesOutput(t *testing.T) {
	// Test case from issue: newSlice contains [4, 4, 5] and oldSlice is [1]
	// added should return [4, 5], not [4, 4, 5]
	t.Run("deduplicates added", func(t *testing.T) {
		added, removed := DiffSlice([]int{1}, []int{4, 4, 5})
		assert.Equal(t, []int{4, 5}, added)
		assert.Equal(t, []int{1}, removed)
	})

	t.Run("deduplicates removed", func(t *testing.T) {
		added, removed := DiffSlice([]int{1, 1, 2}, []int{3})
		assert.Equal(t, []int{3}, added)
		assert.Equal(t, []int{1, 2}, removed)
	})

	t.Run("deduplicates both", func(t *testing.T) {
		added, removed := DiffSlice([]int{1, 1, 2, 2}, []int{3, 3, 4, 4})
		assert.Equal(t, []int{3, 4}, added)
		assert.Equal(t, []int{1, 2}, removed)
	})
}
