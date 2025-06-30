// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUploadFileName(t *testing.T) {
	cases := []struct {
		input, expected string
	}{
		{"", ""},
		{".", ""},
		{"a/./b", "a/b"},
		{"a.git", "a.git"},
		{".git/b", ""},
		{"a/.git", ""},
		{"/a/../../b", "b"},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, CleanGitTreePath(c.input), "input: %q", c.input)
	}
}
