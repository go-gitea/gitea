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

func TestIsEqualSlice(t *testing.T) {
	assert.True(t, IsEqualSlice([]int{2, 0, 2, 3}, []int{2, 0, 2, 3}))
	assert.True(t, IsEqualSlice([]int{3, 0, 2, 2}, []int{3, 0, 2, 2}))
	assert.True(t, IsEqualSlice([]int{}, []int{}))
	assert.True(t, IsEqualSlice([]int(nil), nil))
	assert.True(t, IsEqualSlice([]int(nil), []int{}))
	assert.True(t, IsEqualSlice([]int{}, []int{}))

	assert.True(t, IsEqualSlice([]string{"2", "0", "2", "3"}, []string{"2", "0", "2", "3"}))
	assert.True(t, IsEqualSlice([]float64{2, 0, 2, 3}, []float64{2, 0, 2, 3}))
	assert.True(t, IsEqualSlice([]bool{false, true, false}, []bool{false, true, false}))

	assert.False(t, IsEqualSlice([]int{2, 0, 2}, []int{2, 0, 2, 3}))
	assert.False(t, IsEqualSlice([]int{2, 0, 2, 4}, []int{2, 0, 2, 3}))
	assert.False(t, IsEqualSlice([]int{2, 0, 0, 3}, []int{2, 0, 2, 3}))
}

func TestRemoveFromSlice(t *testing.T) {
	assert.Equal(t, RemoveFromSlice(0, []int{2, 0, 2, 3}), []int{2, 2, 3})
	assert.Equal(t, RemoveFromSlice(2, []int{2, 0, 2, 3}), []int{0, 3})
	assert.Equal(t, RemoveFromSlice(0, []int{0, 0, 0, 0}), []int{})
	assert.Equal(t, RemoveFromSlice(4, []int{2, 0, 2, 3}), []int{2, 0, 2, 3})
	assert.Equal(t, RemoveFromSlice(0, []int{}), []int{})
	assert.Equal(t, RemoveFromSlice(0, []int(nil)), []int(nil))
	assert.Equal(t, RemoveFromSlice(0, []int{}), []int{})

	assert.Equal(t, RemoveFromSlice("0", []string{"2", "0", "2", "3"}), []string{"2", "2", "3"})
	assert.Equal(t, RemoveFromSlice(0, []float64{2, 0, 2, 3}), []float64{2, 2, 3})
	assert.Equal(t, RemoveFromSlice(true, []bool{false, true, false}), []bool{false, false})
}
