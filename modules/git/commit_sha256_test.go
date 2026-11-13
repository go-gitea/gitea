// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFullCommitIDSha256(t *testing.T) {
	id, err := GetFullCommitID(t.Context(), mockRepository("repo1_bare_sha256"), "f004f4")
	assert.NoError(t, err)
	assert.Equal(t, "f004f41359117d319dedd0eaab8c5259ee2263da839dcba33637997458627fdc", id)
}

func TestGetFullCommitIDErrorSha256(t *testing.T) {
	id, err := GetFullCommitID(t.Context(), mockRepository("repo1_bare_sha256"), "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}

func TestHasPreviousCommitSha256(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare_sha256")

	repo, err := OpenRepositoryLocal(t.Context(), bareRepo1Path)
	assert.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetCommit(t.Context(), "f004f41359117d319dedd0eaab8c5259ee2263da839dcba33637997458627fdc")
	assert.NoError(t, err)

	objectFormat, err := repo.GetObjectFormat(t.Context())
	assert.NoError(t, err)

	parentSHA := MustIDFromString("b0ec7af4547047f12d5093e37ef8f1b3b5415ed8ee17894d43a34d7d34212e9c")
	notParentSHA := MustIDFromString("42e334efd04cd36eea6da0599913333c26116e1a537ca76e5b6e4af4dda00236")
	assert.Equal(t, objectFormat, parentSHA.Type())
	assert.Equal(t, "sha256", objectFormat.Name())

	haz, err := commit.HasPreviousCommit(t.Context(), repo, parentSHA)
	assert.NoError(t, err)
	assert.True(t, haz)

	hazNot, err := commit.HasPreviousCommit(t.Context(), repo, notParentSHA)
	assert.NoError(t, err)
	assert.False(t, hazNot)

	selfNot, err := commit.HasPreviousCommit(t.Context(), repo, commit.ID)
	assert.NoError(t, err)
	assert.False(t, selfNot)
}
