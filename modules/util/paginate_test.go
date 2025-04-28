// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginateSlice(t *testing.T) {
	stringSlice := []string{"a", "b", "c", "d", "e"}
	result, ok := PaginateSlice(stringSlice, 1, 2).([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, result)

	result, ok = PaginateSlice(stringSlice, 100, 2).([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{}, result)

	result, ok = PaginateSlice(stringSlice, 3, 2).([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"e"}, result)

	result, ok = PaginateSlice(stringSlice, 1, 0).([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, result)

	result, ok = PaginateSlice(stringSlice, 1, -1).([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, result)

	type Test struct {
		Val int
	}

	testVar := []*Test{{Val: 2}, {Val: 3}, {Val: 4}}
	testVar, ok = PaginateSlice(testVar, 1, 50).([]*Test)
	assert.True(t, ok)
	assert.Equal(t, []*Test{{Val: 2}, {Val: 3}, {Val: 4}}, testVar)

	testVar, ok = PaginateSlice(testVar, 2, 2).([]*Test)
	assert.True(t, ok)
	assert.Equal(t, []*Test{{Val: 4}}, testVar)
}
