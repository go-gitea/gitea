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

func TestGetLatestCommitTime(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}
	lct, err := GetLatestCommitTime(t.Context(), bareRepo1)
	assert.NoError(t, err)
	// Time is Sun Nov 13 16:40:14 2022 +0100
	// which is the time of commit
	// ce064814f4a0d337b333e646ece456cd39fab612 (refs/heads/master)
	assert.EqualValues(t, 1668354014, lct.Unix())
}
