// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
)

func TestReadPatch(t *testing.T) {
	// Ensure we can read the patch files
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	repo, err := OpenRepository(t.Context(), bareRepo1Path)
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

	repo, err := OpenRepository(t.Context(), clonedPath)
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
	_, _, err = gitcmd.NewCommand("update-ref").
		AddDynamicArguments(PullPrefix+"1/head", newCommit).
		WithDir(repo.Path).
		RunStdString(t.Context())
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
	_, _, err = gitcmd.NewCommand("update-ref", "--no-deref", "-d").
		AddDynamicArguments(PullPrefix + "1/head").
		WithDir(repo.Path).
		RunStdString(t.Context())
	assert.NoError(t, err)
}
