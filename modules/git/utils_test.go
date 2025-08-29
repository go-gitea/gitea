// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashFilePathForWebUI(t *testing.T) {
	assert.Equal(t,
		"8843d7f92416211de9ebb963ff4ce28125932878",
		HashFilePathForWebUI("foobar"),
	)
}

func TestSplitCommitTitleBody(t *testing.T) {
	title, body := SplitCommitTitleBody("啊bcdefg", 4)
	assert.Equal(t, "啊…", title)
	assert.Equal(t, "…bcdefg", body)

	title, body = SplitCommitTitleBody("abcdefg\n1234567", 4)
	assert.Equal(t, "a…", title)
	assert.Equal(t, "…bcdefg\n1234567", body)

	title, body = SplitCommitTitleBody("abcdefg\n1234567", 100)
	assert.Equal(t, "abcdefg", title)
	assert.Equal(t, "1234567", body)
}
