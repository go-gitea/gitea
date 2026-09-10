// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginateSlice(t *testing.T) {
	stringSlice := []string{"a", "b", "c", "d", "e"}
	result := PaginateSlice(stringSlice, 1, 2)
	assert.Equal(t, []string{"a", "b"}, result)

	result = PaginateSlice(stringSlice, 100, 2)
	assert.Equal(t, []string{}, result)

	result = PaginateSlice(stringSlice, 3, 2)
	assert.Equal(t, []string{"e"}, result)

	result = PaginateSlice(stringSlice, 1, 0)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, result)

	result = PaginateSlice(stringSlice, 1, -1)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, result)

	type Test struct {
		Val int
	}

	testVar := []*Test{{Val: 2}, {Val: 3}, {Val: 4}}
	testVar = PaginateSlice(testVar, 1, 50)
	assert.Equal(t, []*Test{{Val: 2}, {Val: 3}, {Val: 4}}, testVar)

	testVar = PaginateSlice(testVar, 2, 2)
	assert.Equal(t, []*Test{{Val: 4}}, testVar)
}
