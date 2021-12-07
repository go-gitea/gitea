// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitString(t *testing.T) {
	type testCase struct {
		input    string
		n        int
		leftSub  string
		ellipsis string
	}

	test := func(tc []*testCase, f func(input string, n int) (left, right string)) {
		for _, c := range tc {
			l, r := f(c.input, c.n)
			assert.Equal(t, c.leftSub+c.ellipsis, l, "test split %s at %d, expected leftSub: %s", c.input, c.n, c.leftSub)
			assert.Equal(t, c.ellipsis+c.input[len(c.leftSub):], r, "test split %s at %d, expected rightSub: %s", c.input, c.n, c.input[len(c.leftSub):])
		}
	}

	tc := []*testCase{
		{"abc123xyz", 0, "", utf8Ellipsis},
		{"abc123xyz", 1, "", utf8Ellipsis},
		{"abc123xyz", 4, "a", utf8Ellipsis},
		{"啊bc123xyz", 4, "", utf8Ellipsis},
		{"啊bc123xyz", 6, "啊", utf8Ellipsis},
	}
	test(tc, SplitStringAtByteN)

	tc = []*testCase{
		{"abc123xyz", 0, "", utf8Ellipsis},
		{"abc123xyz", 1, "", utf8Ellipsis},
		{"abc123xyz", 4, "abc", utf8Ellipsis},
		{"啊bc123xyz", 4, "啊bc", utf8Ellipsis},
		{"啊bc123xyz", 6, "啊bc12", utf8Ellipsis},
	}
	test(tc, SplitStringAtRuneN)
}
