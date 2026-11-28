// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginateSliceLargePage(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	list := []int{1, 2, 3, 4, 5}

	tests := []struct {
		name     string
		page     int
		pageSize int
		want     []int
	}{
		{"large page even size", maxInt, 2, []int{}},
		{"large page large size", maxInt, maxInt, []int{}},
		{"second page larger size", 2, maxInt, []int{}},
		{"exact multiple last page", 3, 2, []int{5}},
		{"exact multiple past end", 4, 2, []int{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, PaginateSlice(list, test.page, test.pageSize))
		})
	}

	assert.Equal(t, []int{}, PaginateSlice([]int{}, 1, 2))
}

func TestPaginateSlice(t *testing.T) {
	stringSlice := []string{"a", "b", "c", "d", "e"}
	result := PaginateSlice(stringSlice, 1, 2)
	assert.Equal(t, []string{"a", "b"}, result)

	result = PaginateSlice(stringSlice, 100, 2)
	assert.Equal(t, []string{}, result)

	result = PaginateSlice(stringSlice, int(^uint(0)>>1), 2)
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
