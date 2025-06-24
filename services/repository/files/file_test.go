// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUploadFileName(t *testing.T) {
	assert.Empty(t, CleanGitTreePath(""))
	assert.Empty(t, CleanGitTreePath("."))
	assert.Equal(t, "a/b", CleanGitTreePath("a/b"))
	assert.Empty(t, CleanGitTreePath(".git/b"))
	assert.Empty(t, CleanGitTreePath("a/.git"))
}
