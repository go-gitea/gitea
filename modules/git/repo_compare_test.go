// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFormatPatch(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	clonedPath, err := cloneRepo(t, bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	repo, err := openRepositoryWithDefaultContext(clonedPath)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer repo.Close()

	rd := &bytes.Buffer{}
	err = repo.GetPatch("8d92fc95^...8d92fc95", rd)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	patchb, err := io.ReadAll(rd)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	patch := string(patchb)
	assert.Regexp(t, "^From 8d92fc95", patch)
	assert.Contains(t, patch, "Subject: [PATCH] Add file2.txt")
}

func TestReadPatch(t *testing.T) {
	// Ensure we can read the patch files
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := openRepositoryWithDefaultContext(bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer repo.Close()
	// This patch doesn't exist
	noFile, err := repo.ReadPatchCommit(0)
	assert.Error(t, err)

	// This patch is an empty one (sometimes it's a 404)
	noCommit, err := repo.ReadPatchCommit(1)
	assert.Error(t, err)

	// This patch is legit and should return a commit
	oldCommit, err := repo.ReadPatchCommit(2)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	assert.Empty(t, noFile)
	assert.Empty(t, noCommit)
	assert.Len(t, oldCommit, 40)
	assert.Equal(t, "6e8e2a6f9efd71dbe6917816343ed8415ad696c3", oldCommit)
}

func TestReadWritePullHead(t *testing.T) {
	// Ensure we can write SHA1 head corresponding to PR and open them
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	// As we are writing we should clone the repository first
	clonedPath, err := cloneRepo(t, bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	repo, err := openRepositoryWithDefaultContext(clonedPath)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	defer repo.Close()

	// Try to open non-existing Pull
	_, err = repo.GetRefCommitID(PullPrefix + "0/head")
	assert.Error(t, err)

	// Write a fake sha1 with only 40 zeros
	newCommit := "feaf4ba6bc635fec442f46ddd4512416ec43c2c2"
	err = repo.SetReference(PullPrefix+"1/head", newCommit)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	// Read the file created
	headContents, err := repo.GetRefCommitID(PullPrefix + "1/head")
	if err != nil {
		assert.NoError(t, err)
		return
	}

	assert.Len(t, headContents, 40)
	assert.Equal(t, headContents, newCommit)

	// Remove file after the test
	err = repo.RemoveReference(PullPrefix + "1/head")
	assert.NoError(t, err)
}

func TestGetCommitFilesChanged(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := openRepositoryWithDefaultContext(bareRepo1Path)
	assert.NoError(t, err)
	defer repo.Close()

	objectFormat, err := repo.GetObjectFormat()
	assert.NoError(t, err)

	testCases := []struct {
		base, head string
		files      []string
	}{
		{
			objectFormat.EmptyObjectID().String(),
			"95bb4d39648ee7e325106df01a621c530863a653",
			[]string{"file1.txt"},
		},
		{
			objectFormat.EmptyObjectID().String(),
			"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			[]string{"file2.txt"},
		},
		{
			"95bb4d39648ee7e325106df01a621c530863a653",
			"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			[]string{"file2.txt"},
		},
		{
			objectFormat.EmptyTree().String(),
			"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			[]string{"file1.txt", "file2.txt"},
		},
	}

	for _, tc := range testCases {
		changedFiles, err := repo.GetFilesChangedBetween(tc.base, tc.head)
		assert.NoError(t, err)
		assert.ElementsMatch(t, tc.files, changedFiles)
	}
}
