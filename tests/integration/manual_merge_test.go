// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManualMergeAutodetect(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// user2 is the repo owner
		// user1 is the pusher/merger
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session2 := loginUser(t, user2.Name)

		// Create a repo owned by user2
		repoName := "manual-merge-autodetect"
		defaultBranch := setting.Repository.DefaultBranch
		user2Ctx := NewAPITestContext(t, user2.Name, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(user2Ctx, false)(t)

		// Enable autodetect manual merge
		doAPIEditRepository(user2Ctx, &api.EditRepoOption{
			HasPullRequests:       new(true),
			AllowManualMerge:      new(true),
			AutodetectManualMerge: new(true),
		})(t)

		// Create a PR from a branch
		branchName := "feature"
		testEditFileToNewBranch(t, session2, user2.Name, repoName, defaultBranch, branchName, "README.md", "Manual Merge Test")

		apiPull, err := doAPICreatePullRequest(NewAPITestContext(t, user1.Name, repoName, auth_model.AccessTokenScopeWriteRepository), user2.Name, repoName, defaultBranch, branchName)(t)
		assert.NoError(t, err)

		// user1 clones and pushes the branch to master (fast-forward)
		dstPath := t.TempDir()
		u, _ := url.Parse(giteaURL.String())
		u.Path = fmt.Sprintf("%s/%s.git", user2.Name, repoName)
		u.User = url.UserPassword(user1.Name, userPassword)

		doGitClone(dstPath, u)(t)
		doGitMerge(dstPath, "origin/"+branchName)(t)
		doGitPushTestRepository(dstPath, "origin", defaultBranch)(t)

		// Wait for the PR to be marked as merged by the background task
		var pr *issues_model.PullRequest
		require.Eventually(t, func() bool {
			pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
			return pr.HasMerged
		}, 10*time.Second, 100*time.Millisecond)

		// Check if the PR is merged and who is the merger
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
		assert.True(t, pr.HasMerged)
		assert.Equal(t, issues_model.PullRequestStatusManuallyMerged, pr.Status)
		// Merger should be user1 (the pusher), not the commit author (user2) or repo owner (user2)
		assert.Equal(t, user1.ID, pr.MergerID)
	})
}

// TestManualMergeAutodetectMultiplePRs covers the case where several PRs targeting
// the same base branch are merged locally in sequence and pushed in a single push.
//
// Reproduces the regression where only the last PR's merge commit is detected.
// The "git rev-list --ancestry-path --merges --reverse <prHead>..<base>" output
// in services/pull/check.go:getMergeCommit returns multiple lines for any PR that
// is not the most recent merge, and the multi-line value is forwarded to
// gitRepo.GetCommit, which (since Gitea moved to "git cat-file --batch-command"
// in #35775) kills the repository's cached cat-file process and breaks the
// follow-up commit lookup.
func TestManualMergeAutodetectMultiplePRs(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// user2 is the repo owner
		// user1 is the pusher/merger
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session2 := loginUser(t, user2.Name)

		// Create a repo owned by user2
		repoName := "manual-merge-autodetect-multi"
		defaultBranch := setting.Repository.DefaultBranch
		user2Ctx := NewAPITestContext(t, user2.Name, repoName, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		doAPICreateRepository(user2Ctx, false)(t)

		// Enable autodetect manual merge
		doAPIEditRepository(user2Ctx, &api.EditRepoOption{
			HasPullRequests:       new(true),
			AllowManualMerge:      new(true),
			AutodetectManualMerge: new(true),
		})(t)

		// Create three branches from the default branch, each adding its own file,
		// and open a PR for each of them targeting the default branch. Each branch
		// touches a distinct file so the sequential merges don't conflict.
		branchNames := []string{"fix-1", "fix-2", "fix-3"}
		apiPulls := make([]api.PullRequest, len(branchNames))
		for i, branchName := range branchNames {
			testCreateFile(t, session2, user2.Name, repoName, defaultBranch, branchName,
				fmt.Sprintf("file-%d.txt", i+1), fmt.Sprintf("manual merge multi-PR test %d", i+1))

			pr, err := doAPICreatePullRequest(
				NewAPITestContext(t, user1.Name, repoName, auth_model.AccessTokenScopeWriteRepository),
				user2.Name, repoName, defaultBranch, branchName,
			)(t)
			require.NoError(t, err)
			apiPulls[i] = pr
		}

		// user1 clones, then merges every branch sequentially, then pushes once.
		// The first merge fast-forwards; the rest produce real merge commits, which
		// is exactly the situation that breaks "ancestry-path --merges --reverse".
		dstPath := t.TempDir()
		u, _ := url.Parse(giteaURL.String())
		u.Path = fmt.Sprintf("%s/%s.git", user2.Name, repoName)
		u.User = url.UserPassword(user1.Name, userPassword)

		doGitClone(dstPath, u)(t)
		// Set a committer/author on the local clone so "git merge" can create merge
		// commits without falling back to a global identity.
		_, _, err := gitcmd.NewCommand("config", "user.email").AddDynamicArguments(user1.Email).
			WithDir(dstPath).RunStdString(t.Context())
		require.NoError(t, err)
		_, _, err = gitcmd.NewCommand("config", "user.name").AddDynamicArguments(user1.Name).
			WithDir(dstPath).RunStdString(t.Context())
		require.NoError(t, err)

		// Capture each branch's expected merge commit hash from the local clone,
		// so we can assert that Gitea recorded the correct merge commit per PR
		// (and not just "some merge commit" — see the regression where every PR
		// was attributed to the last merge in the push).
		expectedMergeCommits := make([]string, len(branchNames))
		for i, branchName := range branchNames {
			doGitMerge(dstPath, "--no-ff", "-m", "merge "+branchName, "origin/"+branchName)(t)
			head, _, cmdErr := gitcmd.NewCommand("rev-parse", "HEAD").
				WithDir(dstPath).RunStdString(t.Context())
			require.NoError(t, cmdErr)
			expectedMergeCommits[i] = strings.TrimSpace(head)
		}
		doGitPushTestRepository(dstPath, "origin", defaultBranch)(t)

		// Every PR should be detected as manually merged by the background task,
		// not just the last one.
		require.Eventually(t, func() bool {
			for _, apiPull := range apiPulls {
				pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
				if !pr.HasMerged {
					return false
				}
			}
			return true
		}, 15*time.Second, 200*time.Millisecond)

		for i, apiPull := range apiPulls {
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
			assert.Truef(t, pr.HasMerged, "PR %d (%s) should be marked as merged", i+1, branchNames[i])
			assert.Equalf(t, issues_model.PullRequestStatusManuallyMerged, pr.Status,
				"PR %d (%s) should have ManuallyMerged status", i+1, branchNames[i])
			assert.Equalf(t, user1.ID, pr.MergerID,
				"PR %d (%s) merger should be the pusher", i+1, branchNames[i])
			assert.Equalf(t, expectedMergeCommits[i], pr.MergedCommitID,
				"PR %d (%s) should be attributed to its own merge commit, not another PR's", i+1, branchNames[i])
		}
	})
}
