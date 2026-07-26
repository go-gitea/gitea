// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"os"
	"path/filepath"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/require"
)

func TestGitPatchPrepare(t *testing.T) {
	unittest.PrepareTestEnv(t)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	gitRepo, err := git.OpenRepository(repo1.CodeStorageRepo())
	require.NoError(t, err)
	defer gitRepo.Close()

	opts := &ApplyDiffPatchOptions{}
	tmpRepo, err := gitPatchPrepare(t.Context(), repo1, gitRepo, user2, opts)
	require.NoError(t, err)
	defer tmpRepo.Close()
	// temporary repository for patch should not be a bare repo because "--index" argument is used,
	// while it is questionable whether "--index" argument should be really used, need to figure out in the future.
	_, err = os.Stat(filepath.Join(tmpRepo.basePath, ".git"))
	require.NoError(t, err)
}
