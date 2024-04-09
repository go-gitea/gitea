// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterMapUnique(t *testing.T) {
	result := FilterSlice([]int{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	}, func(i int) (int, bool) {
		switch i {
		case 0:
			return 0, true // included later
		case 1:
			return 0, true // duplicate of previous (should be ignored)
		case 2:
			return 2, false // not included
		default:
			return i, true
		}
	})
	assert.Equal(t, []int{0, 3, 4, 5, 6, 7, 8, 9}, result)
}
