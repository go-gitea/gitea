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

	doTest := func(t *testing.T, objectFormatName string) {
		tmpGitRepo, err := NewTemporaryUploadRepository(mockedRepo)
		require.NoError(t, err)
		defer tmpGitRepo.Close()

		require.NoError(t, tmpGitRepo.Init(t.Context(), objectFormatName))

		require.NoError(t, tmpGitRepo.RemoveFilesFromIndex(t.Context(), "any-file-name"))
		require.NoError(t, tmpGitRepo.RemoveFilesFromIndex(t.Context(), "--any-file-name"))

		objID, err := tmpGitRepo.HashObjectAndWrite(t.Context(), bytes.NewReader(nil))
		require.NoError(t, err)
		require.NoError(t, tmpGitRepo.AddObjectToIndex(t.Context(), "100644", objID, "any-file-name"))
		require.NoError(t, tmpGitRepo.AddObjectToIndex(t.Context(), "100644", objID, "--any-file-name"))
	}

	t.Run("sha1", func(t *testing.T) {
		doTest(t, git.Sha1ObjectFormat.Name())
	})

	t.Run("sha256", func(t *testing.T) {
		if !git.DefaultFeatures().SupportHashSha256 {
			t.Skip("sha256 is not supported")
		}
		doTest(t, git.Sha256ObjectFormat.Name())
	})
}
