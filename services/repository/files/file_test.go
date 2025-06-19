// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUploadFileName(t *testing.T) {
	assert.Equal(t, "", CleanGitTreePath(""))  //nolint
	assert.Equal(t, "", CleanGitTreePath(".")) //nolint
	assert.Equal(t, "a/b", CleanGitTreePath("a/b"))
	assert.Equal(t, "", CleanGitTreePath(".git/b")) //nolint
	assert.Equal(t, "", CleanGitTreePath("a/.git")) //nolint
}
