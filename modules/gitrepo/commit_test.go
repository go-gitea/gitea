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

func TestHasPreviousCommitSha256(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare_sha256"}
	newCommitID := "f004f41359117d319dedd0eaab8c5259ee2263da839dcba33637997458627fdc"

	parentSHA := "b0ec7af4547047f12d5093e37ef8f1b3b5415ed8ee17894d43a34d7d34212e9c"
	notParentSHA := "42e334efd04cd36eea6da0599913333c26116e1a537ca76e5b6e4af4dda00236"

	haz, err := HasPreviousCommit(t.Context(), repo, newCommitID, parentSHA)
	assert.NoError(t, err)
	assert.True(t, haz)

	hazNot, err := HasPreviousCommit(t.Context(), repo, newCommitID, notParentSHA)
	assert.NoError(t, err)
	assert.False(t, hazNot)

	selfNot, err := HasPreviousCommit(t.Context(), repo, newCommitID, newCommitID)
	assert.NoError(t, err)
	assert.False(t, selfNot)
}

func TestHasPreviousCommit(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}

	newCommitID := "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"

	parentSHA := "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2"
	notParentSHA := "2839944139e0de9737a044f78b0e4b40d989a9e3"

	haz, err := HasPreviousCommit(t.Context(), repo, newCommitID, parentSHA)
	assert.NoError(t, err)
	assert.True(t, haz)

	hazNot, err := HasPreviousCommit(t.Context(), repo, newCommitID, notParentSHA)
	assert.NoError(t, err)
	assert.False(t, hazNot)

	selfNot, err := HasPreviousCommit(t.Context(), repo, newCommitID, newCommitID)
	assert.NoError(t, err)
	assert.False(t, selfNot)
}
