// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetCommitBranches(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	// these test case are specific to the repo1_bare test repo
	testCases := []struct {
		CommitID         string
		ExpectedBranches []string
	}{
		{"2839944139e0de9737a044f78b0e4b40d989a9e3", []string{"branch1"}},
		{"5c80b0245c1c6f8343fa418ec374b13b5d4ee658", []string{"branch2"}},
		{"37991dec2c8e592043f47155ce4808d4580f9123", []string{"master"}},
		{"95bb4d39648ee7e325106df01a621c530863a653", []string{"branch1", "branch2"}},
		{"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2", []string{"branch2", "master"}},
		{"master", []string{"master"}},
	}
	for _, testCase := range testCases {
		commit, err := bareRepo1.GetCommit(testCase.CommitID)
		assert.NoError(t, err)
		branches, err := bareRepo1.getBranches(commit, 2)
		assert.NoError(t, err)
		assert.Equal(t, testCase.ExpectedBranches, branches)
	}
}

func TestGetTagCommitWithSignature(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	commit, err := bareRepo1.GetCommit("3ad28a9149a2864384548f3d17ed7f38014c9e8a")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	assert.NotNil(t, commit.Signature)
	// test that signature is not in message
	assert.Equal(t, "tag", commit.CommitMessage)
}

func TestGetCommitWithBadCommitID(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	commit, err := bareRepo1.GetCommit("bad_branch")
	assert.Nil(t, commit)
	assert.Error(t, err)
	assert.True(t, IsErrNotExist(err))
}
