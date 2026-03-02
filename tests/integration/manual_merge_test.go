// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
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
