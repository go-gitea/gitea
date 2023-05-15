// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceContains(t *testing.T) {
	assert.True(t, SliceContains([]int{2, 0, 2, 3}, 2))
	assert.True(t, SliceContains([]int{2, 0, 2, 3}, 0))
	assert.True(t, SliceContains([]int{2, 0, 2, 3}, 3))

	assert.True(t, SliceContains([]string{"2", "0", "2", "3"}, "0"))
	assert.True(t, SliceContains([]float64{2, 0, 2, 3}, 0))
	assert.True(t, SliceContains([]bool{false, true, false}, true))

	assert.False(t, SliceContains([]int{2, 0, 2, 3}, 4))
	assert.False(t, SliceContains([]int{}, 4))
	assert.False(t, SliceContains(nil, 4))
}

func TestSliceContainsString(t *testing.T) {
	assert.True(t, SliceContainsString([]string{"c", "b", "a", "b"}, "a"))
	assert.True(t, SliceContainsString([]string{"c", "b", "a", "b"}, "b"))
	assert.True(t, SliceContainsString([]string{"c", "b", "a", "b"}, "A", true))
	assert.True(t, SliceContainsString([]string{"C", "B", "A", "B"}, "a", true))

	assert.False(t, SliceContainsString([]string{"c", "b", "a", "b"}, "z"))
	assert.False(t, SliceContainsString([]string{"c", "b", "a", "b"}, "A"))
	assert.False(t, SliceContainsString([]string{}, "a"))
	assert.False(t, SliceContainsString(nil, "a"))
}

func TestSliceSortedEqual(t *testing.T) {
	assert.True(t, SliceSortedEqual([]int{2, 0, 2, 3}, []int{2, 0, 2, 3}))
	assert.True(t, SliceSortedEqual([]int{3, 0, 2, 2}, []int{2, 0, 2, 3}))
	assert.True(t, SliceSortedEqual([]int{}, []int{}))
	assert.True(t, SliceSortedEqual([]int(nil), nil))
	assert.True(t, SliceSortedEqual([]int(nil), []int{}))
	assert.True(t, SliceSortedEqual([]int{}, []int{}))

	assert.True(t, SliceSortedEqual([]string{"2", "0", "2", "3"}, []string{"2", "0", "2", "3"}))
	assert.True(t, SliceSortedEqual([]float64{2, 0, 2, 3}, []float64{2, 0, 2, 3}))
	assert.True(t, SliceSortedEqual([]bool{false, true, false}, []bool{false, true, false}))

	assert.False(t, SliceSortedEqual([]int{2, 0, 2}, []int{2, 0, 2, 3}))
	assert.False(t, SliceSortedEqual([]int{}, []int{2, 0, 2, 3}))
	assert.False(t, SliceSortedEqual(nil, []int{2, 0, 2, 3}))
	assert.False(t, SliceSortedEqual([]int{2, 0, 2, 4}, []int{2, 0, 2, 3}))
	assert.False(t, SliceSortedEqual([]int{2, 0, 0, 3}, []int{2, 0, 2, 3}))
}

func TestSliceEqual(t *testing.T) {
	assert.True(t, SliceEqual([]int{2, 0, 2, 3}, []int{2, 0, 2, 3}))
	assert.True(t, SliceEqual([]int{}, []int{}))
	assert.True(t, SliceEqual([]int(nil), nil))
	assert.True(t, SliceEqual([]int(nil), []int{}))
	assert.True(t, SliceEqual([]int{}, []int{}))

	assert.True(t, SliceEqual([]string{"2", "0", "2", "3"}, []string{"2", "0", "2", "3"}))
	assert.True(t, SliceEqual([]float64{2, 0, 2, 3}, []float64{2, 0, 2, 3}))
	assert.True(t, SliceEqual([]bool{false, true, false}, []bool{false, true, false}))

	assert.False(t, SliceEqual([]int{3, 0, 2, 2}, []int{2, 0, 2, 3}))
	assert.False(t, SliceEqual([]int{2, 0, 2}, []int{2, 0, 2, 3}))
	assert.False(t, SliceEqual([]int{}, []int{2, 0, 2, 3}))
	assert.False(t, SliceEqual(nil, []int{2, 0, 2, 3}))
	assert.False(t, SliceEqual([]int{2, 0, 2, 4}, []int{2, 0, 2, 3}))
	assert.False(t, SliceEqual([]int{2, 0, 0, 3}, []int{2, 0, 2, 3}))
}

func TestSliceRemoveAll(t *testing.T) {
	assert.ElementsMatch(t, []int{2, 2, 3}, SliceRemoveAll([]int{2, 0, 2, 3}, 0))
	assert.ElementsMatch(t, []int{0, 3}, SliceRemoveAll([]int{2, 0, 2, 3}, 2))
	assert.Empty(t, SliceRemoveAll([]int{0, 0, 0, 0}, 0))
	assert.ElementsMatch(t, []int{2, 0, 2, 3}, SliceRemoveAll([]int{2, 0, 2, 3}, 4))
	assert.Empty(t, SliceRemoveAll([]int{}, 0))
	assert.ElementsMatch(t, []int(nil), SliceRemoveAll([]int(nil), 0))
	assert.Empty(t, SliceRemoveAll([]int{}, 0))

	assert.ElementsMatch(t, []string{"2", "2", "3"}, SliceRemoveAll([]string{"2", "0", "2", "3"}, "0"))
	assert.ElementsMatch(t, []float64{2, 2, 3}, SliceRemoveAll([]float64{2, 0, 2, 3}, 0))
	assert.ElementsMatch(t, []bool{false, false}, SliceRemoveAll([]bool{false, true, false}, true))
}
