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

// repo7_rename has 2 commits, the first adds a.txt and the second rename a.txt to b.txt

func TestFileCommitsCountWithoutRename(t *testing.T) {
	renameRepo7 := &mockRepository{path: "repo7_rename"}

	commitsCount, err := CommitsCount(t.Context(), renameRepo7,
		CommitsCountOptions{
			Revision:     []string{"05f331b6ef83f1d02b42ee0fefe28e321cf94e8c"},
			RelPath:      []string{"b.txt"},
			FollowRename: false,
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), commitsCount)
}

func TestFileCommitsCountWithRename(t *testing.T) {
	renameRepo7 := &mockRepository{path: "repo7_rename"}

	commitsCount, err := CommitsCount(t.Context(), renameRepo7,
		CommitsCountOptions{
			Revision:     []string{"05f331b6ef83f1d02b42ee0fefe28e321cf94e8c"},
			RelPath:      []string{"b.txt"},
			FollowRename: true,
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), commitsCount)
}
