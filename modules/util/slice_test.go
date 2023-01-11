// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceContains(t *testing.T) {
	assert.True(t, SliceContains(2, []int{2, 0, 2, 3}))
	assert.True(t, SliceContains(0, []int{2, 0, 2, 3}))
	assert.True(t, SliceContains(3, []int{2, 0, 2, 3}))

	assert.True(t, SliceContains("0", []string{"2", "0", "2", "3"}))
	assert.True(t, SliceContains(0, []float64{2, 0, 2, 3}))
	assert.True(t, SliceContains(true, []bool{false, true, false}))

	assert.False(t, SliceContains(4, []int{2, 0, 2, 3}))
	assert.False(t, SliceContains(4, []int{}))
	assert.False(t, SliceContains(4, nil))
}

func TestSliceContainsString(t *testing.T) {
	assert.True(t, SliceContainsString("a", []string{"c", "b", "a", "b"}))
	assert.True(t, SliceContainsString("b", []string{"c", "b", "a", "b"}))
	assert.True(t, SliceContainsString("A", []string{"c", "b", "a", "b"}, true))
	assert.True(t, SliceContainsString("a", []string{"C", "B", "A", "B"}, true))

	assert.False(t, SliceContainsString("z", []string{"c", "b", "a", "b"}))
	assert.False(t, SliceContainsString("A", []string{"c", "b", "a", "b"}))
	assert.False(t, SliceContainsString("a", []string{}))
	assert.False(t, SliceContainsString("a", nil))
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
