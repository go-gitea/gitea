// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

type mockRepository struct {
	path string
}

func (r *mockRepository) RelativePath() string {
	return r.path
}

func TestRepoGetDivergingCommits(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}
	do, err := GetDivergingCommits(t.Context(), repo, "master", "branch2")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  1,
		Behind: 5,
	}, do)

	do, err = GetDivergingCommits(t.Context(), repo, "master", "master")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  0,
		Behind: 0,
	}, do)

	do, err = GetDivergingCommits(t.Context(), repo, "master", "test")
	assert.NoError(t, err)
	assert.Equal(t, &DivergeObject{
		Ahead:  0,
		Behind: 2,
	}, do)
}

func TestGetCommitFilesChanged(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}

	testCases := []struct {
		base, head string
		files      []string
	}{
		{
			git.Sha1ObjectFormat.EmptyObjectID().String(),
			"95bb4d39648ee7e325106df01a621c530863a653",
			[]string{"file1.txt"},
		},
		{
			git.Sha1ObjectFormat.EmptyObjectID().String(),
			"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			[]string{"file2.txt"},
		},
		{
			"95bb4d39648ee7e325106df01a621c530863a653",
			"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			[]string{"file2.txt"},
		},
		{
			git.Sha1ObjectFormat.EmptyTree().String(),
			"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			[]string{"file1.txt", "file2.txt"},
		},
	}

	for _, tc := range testCases {
		changedFiles, err := GetFilesChangedBetween(t.Context(), repo, tc.base, tc.head)
		assert.NoError(t, err)
		assert.ElementsMatch(t, tc.files, changedFiles)
	}
}
