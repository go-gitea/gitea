// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}

	commitsCount, err := CommitsCount(t.Context(), bareRepo1,
		CommitsCountOptions{
			Revision: []string{"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), commitsCount)
}

func TestCommitsCountWithoutBase(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}

	commitsCount, err := CommitsCount(t.Context(), bareRepo1,
		CommitsCountOptions{
			Not:      "master",
			Revision: []string{"branch1"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), commitsCount)
}
