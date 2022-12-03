// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNaturalSortLess(t *testing.T) {
	test := func(s1, s2 string, less bool) {
		assert.Equal(t, less, NaturalSortLess(s1, s2))
	}
	test("v1.20.0", "v1.2.0", false)
	test("v1.20.0", "v1.29.0", true)
	test("v1.20.0", "v1.20.0", false)
	test("abc", "bcd", true)
	test("a-1-a", "a-1-b", true)
	test("2", "12", true)
	test("a", "ab", true)
}
