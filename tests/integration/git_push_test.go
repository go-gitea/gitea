// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/require"
)

func TestGitPush(t *testing.T) {
	onGiteaRun(t, testGitPush)
}

func testGitPush(t *testing.T, u *url.URL) {
	t.Run("Push branch with options", func(t *testing.T) {
		runTestGitPush(t, u, func(t *testing.T, gitPath string) {
			branchName := "branch-with-options"
			doGitCreateBranch(gitPath, branchName)(t)
			doGitPushTestRepository(gitPath, "origin", branchName, "-o", "repo.private=true", "-o", "repo.template=true")(t)
		})
	})
}

func runTestGitPush(t *testing.T, u *url.URL, gitOperation func(t *testing.T, gitPath string)) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo, err := repo_service.CreateRepository(db.DefaultContext, user, user, repo_service.CreateRepoOptions{
		Name:          "repo-to-push",
		Description:   "test git push",
		AutoInit:      false,
		DefaultBranch: "main",
		IsPrivate:     false,
	})
	require.NoError(t, err)
	require.NotEmpty(t, repo)

	gitPath := t.TempDir()

	doGitInitTestRepository(gitPath)(t)

	oldPath := u.Path
	oldUser := u.User
	defer func() {
		u.Path = oldPath
		u.User = oldUser
	}()
	u.Path = repo.FullName() + ".git"
	u.User = url.UserPassword(user.LowerName, userPassword)

	doGitAddRemote(gitPath, "origin", u)(t)

	gitRepo, err := git.OpenRepository(git.DefaultContext, gitPath)
	require.NoError(t, err)
	defer gitRepo.Close()

	gitOperation(t, gitPath)

	require.NoError(t, repo_service.DeleteRepositoryDirectly(db.DefaultContext, user, user.ID, repo.ID))
}
