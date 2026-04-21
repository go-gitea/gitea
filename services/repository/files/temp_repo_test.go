// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"bytes"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/require"
)

func TestTemporaryUploadRepository(t *testing.T) {
	mockedRepo := &repo_model.Repository{Name: "mocked-repo-name", OwnerName: "mocked-owner-name"}
	tmpGitRepo, err := NewTemporaryUploadRepository(mockedRepo)
	require.NoError(t, err)
	defer tmpGitRepo.Close()

	require.NoError(t, tmpGitRepo.Init(t.Context(), git.Sha256ObjectFormat.Name()))

	require.NoError(t, tmpGitRepo.RemoveFilesFromIndex(t.Context(), "any-file-name"))
	require.NoError(t, tmpGitRepo.RemoveFilesFromIndex(t.Context(), "--any-file-name"))

	objID, err := tmpGitRepo.HashObjectAndWrite(t.Context(), bytes.NewReader(nil))
	require.NoError(t, err)
	require.NoError(t, tmpGitRepo.AddObjectToIndex(t.Context(), "100644", objID, "any-file-name"))
	require.NoError(t, tmpGitRepo.AddObjectToIndex(t.Context(), "100644", objID, "--any-file-name"))
}
