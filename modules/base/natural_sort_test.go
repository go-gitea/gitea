// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNaturalSortLess(t *testing.T) {
	testLess := func(s1, s2 string) {
		assert.True(t, NaturalSortLess(s1, s2), "s1<s2 should be true: s1=%q, s2=%q", s1, s2)
		assert.False(t, NaturalSortLess(s2, s1), "s2<s1 should be false: s1=%q, s2=%q", s1, s2)
	}
	testEqual := func(s1, s2 string) {
		assert.False(t, NaturalSortLess(s1, s2), "s1<s2 should be false: s1=%q, s2=%q", s1, s2)
		assert.False(t, NaturalSortLess(s2, s1), "s2<s1 should be false: s1=%q, s2=%q", s1, s2)
	}

	testEqual("", "")
	testLess("", "a")
	testLess("", "1")

	testLess("v1.2", "v1.2.0")
	testLess("v1.2.0", "v1.10.0")
	testLess("v1.20.0", "v1.29.0")
	testEqual("v1.20.0", "v1.20.0")

	testLess("a", "A")
	testLess("a", "B")
	testLess("A", "b")
	testLess("A", "ab")

	testLess("abc", "bcd")
	testLess("a-1-a", "a-1-b")
	testLess("2", "12")

	testLess("cafe", "café")
	testLess("café", "caff")

	testLess("A-2", "A-11")
	testLess("0.txt", "1.txt")
}
