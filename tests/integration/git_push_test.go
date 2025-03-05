// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitPush(t *testing.T) {
	onGiteaRun(t, testGitPush)
}

func testGitPush(t *testing.T, u *url.URL) {
	t.Run("Push branches at once", func(t *testing.T) {
		runTestGitPush(t, u, func(t *testing.T, gitPath string) (pushed, deleted []string) {
			for i := 0; i < 100; i++ {
				branchName := fmt.Sprintf("branch-%d", i)
				pushed = append(pushed, branchName)
				doGitCreateBranch(gitPath, branchName)(t)
			}
			pushed = append(pushed, "master")
			doGitPushTestRepository(gitPath, "origin", "--all")(t)
			return pushed, deleted
		})
	})

	t.Run("Push branches exists", func(t *testing.T) {
		runTestGitPush(t, u, func(t *testing.T, gitPath string) (pushed, deleted []string) {
			for i := 0; i < 10; i++ {
				branchName := fmt.Sprintf("branch-%d", i)
				if i < 5 {
					pushed = append(pushed, branchName)
				}
				doGitCreateBranch(gitPath, branchName)(t)
			}
			// only push master and the first 5 branches
			pushed = append(pushed, "master")
			args := append([]string{"origin"}, pushed...)
			doGitPushTestRepository(gitPath, args...)(t)

			pushed = pushed[:0]
			// do some changes for the first 5 branches created above
			for i := 0; i < 5; i++ {
				branchName := fmt.Sprintf("branch-%d", i)
				pushed = append(pushed, branchName)

				doGitAddSomeCommits(gitPath, branchName)(t)
			}

			for i := 5; i < 10; i++ {
				pushed = append(pushed, fmt.Sprintf("branch-%d", i))
			}
			pushed = append(pushed, "master")

			// push all, so that master are not chagned
			doGitPushTestRepository(gitPath, "origin", "--all")(t)

			return pushed, deleted
		})
	})

	t.Run("Push branches one by one", func(t *testing.T) {
		runTestGitPush(t, u, func(t *testing.T, gitPath string) (pushed, deleted []string) {
			for i := 0; i < 100; i++ {
				branchName := fmt.Sprintf("branch-%d", i)
				doGitCreateBranch(gitPath, branchName)(t)
				doGitPushTestRepository(gitPath, "origin", branchName)(t)
				pushed = append(pushed, branchName)
			}
			return pushed, deleted
		})
	})

	t.Run("Push branch with options", func(t *testing.T) {
		runTestGitPush(t, u, func(t *testing.T, gitPath string) (pushed, deleted []string) {
			branchName := "branch-with-options"
			doGitCreateBranch(gitPath, branchName)(t)
			doGitPushTestRepository(gitPath, "origin", branchName, "-o", "repo.private=true", "-o", "repo.template=true")(t)
			pushed = append(pushed, branchName)

			return pushed, deleted
		})
	})

	t.Run("Delete branches", func(t *testing.T) {
		runTestGitPush(t, u, func(t *testing.T, gitPath string) (pushed, deleted []string) {
			doGitPushTestRepository(gitPath, "origin", "master")(t) // make sure master is the default branch instead of a branch we are going to delete
			pushed = append(pushed, "master")

			for i := 0; i < 100; i++ {
				branchName := fmt.Sprintf("branch-%d", i)
				pushed = append(pushed, branchName)
				doGitCreateBranch(gitPath, branchName)(t)
			}
			doGitPushTestRepository(gitPath, "origin", "--all")(t)

			for i := 0; i < 10; i++ {
				branchName := fmt.Sprintf("branch-%d", i)
				doGitPushTestRepository(gitPath, "origin", "--delete", branchName)(t)
				deleted = append(deleted, branchName)
			}
			return pushed, deleted
		})
	})

	t.Run("Push to deleted branch", func(t *testing.T) {
		runTestGitPush(t, u, func(t *testing.T, gitPath string) (pushed, deleted []string) {
			doGitPushTestRepository(gitPath, "origin", "master")(t) // make sure master is the default branch instead of a branch we are going to delete
			pushed = append(pushed, "master")

			doGitCreateBranch(gitPath, "branch-1")(t)
			doGitPushTestRepository(gitPath, "origin", "branch-1")(t)
			pushed = append(pushed, "branch-1")

			// delete and restore
			doGitPushTestRepository(gitPath, "origin", "--delete", "branch-1")(t)
			doGitPushTestRepository(gitPath, "origin", "branch-1")(t)

			return pushed, deleted
		})
	})
}

func runTestGitPush(t *testing.T, u *url.URL, gitOperation func(t *testing.T, gitPath string) (pushed, deleted []string)) {
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

	pushedBranches, deletedBranches := gitOperation(t, gitPath)

	dbBranches := make([]*git_model.Branch, 0)
	require.NoError(t, db.GetEngine(db.DefaultContext).Where("repo_id=?", repo.ID).Find(&dbBranches))
	assert.Equalf(t, len(pushedBranches), len(dbBranches), "mismatched number of branches in db")
	dbBranchesMap := make(map[string]*git_model.Branch, len(dbBranches))
	for _, branch := range dbBranches {
		dbBranchesMap[branch.Name] = branch
	}

	deletedBranchesMap := make(map[string]bool, len(deletedBranches))
	for _, branchName := range deletedBranches {
		deletedBranchesMap[branchName] = true
	}

	for _, branchName := range pushedBranches {
		branch, ok := dbBranchesMap[branchName]
		deleted := deletedBranchesMap[branchName]
		assert.True(t, ok, "branch %s not found in database", branchName)
		assert.Equal(t, deleted, branch.IsDeleted, "IsDeleted of %s is %v, but it's expected to be %v", branchName, branch.IsDeleted, deleted)
		commitID, err := gitRepo.GetBranchCommitID(branchName)
		require.NoError(t, err)
		assert.Equal(t, commitID, branch.CommitID)
	}

	require.NoError(t, repo_service.DeleteRepositoryDirectly(db.DefaultContext, user, repo.ID))
}

func TestPushPullRefs(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		baseAPITestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		u.Path = baseAPITestContext.GitPath()
		u.User = url.UserPassword("user2", userPassword)

		dstPath := t.TempDir()
		doGitClone(dstPath, u)(t)

		cmd := git.NewCommand("push", "--delete", "origin", "refs/pull/2/head")
		stdout, stderr, err := cmd.RunStdString(git.DefaultContext, &git.RunOpts{
			Dir: dstPath,
		})
		assert.Error(t, err)
		assert.Empty(t, stdout)
		assert.NotContains(t, stderr, "[deleted]", "stderr: %s", stderr)
	})
}
