// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

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

func TestGetCommitIDsBetweenReverse(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}

	// tests raw commit IDs
	commitIDs, err := GetCommitIDsBetweenReverse(t.Context(), repo,
		"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
		"",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0",
		"6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1",
		"37991dec2c8e592043f47155ce4808d4580f9123",
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
		"6fbd69e9823458e6c4a2fc5c0f6bc022b2f2acd1",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"37991dec2c8e592043f47155ce4808d4580f9123",
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
		"",
		3,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"37991dec2c8e592043f47155ce4808d4580f9123",
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	// test branch names instead of raw commit IDs.
	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"test",
		"master",
		"",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)

	// add notref to exclude test
	commitIDs, err = GetCommitIDsBetweenReverse(t.Context(), repo,
		"test",
		"master",
		"test",
		100,
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"feaf4ba6bc635fec442f46ddd4512416ec43c2c2",
		"ce064814f4a0d337b333e646ece456cd39fab612",
	}, commitIDs)
}
