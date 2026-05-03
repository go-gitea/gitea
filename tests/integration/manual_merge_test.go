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
	repo_model "code.gitea.io/gitea/models/repo"
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

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: repoName})
		user1Ctx := NewAPITestContext(t, user1.Name, repoName, auth_model.AccessTokenScopeWriteRepository)

		// multiple PRs should be able to be closed together if a push contains their branch commits.
		branchNames := []string{"fix-1", "fix-2"}
		apiPulls := make([]api.PullRequest, len(branchNames))
		for i, branchName := range branchNames {
			_, err := createFileInBranch(user2, repo,
				createFileInBranchOptions{OldBranch: defaultBranch, NewBranch: branchName},
				map[string]string{"test-file-" + branchName: "dummy"},
			)
			require.NoError(t, err)
			apiPulls[i], err = doAPICreatePullRequest(user1Ctx, user2.Name, repoName, defaultBranch, branchName)(t)
			require.NoError(t, err)
		}

		// user1 clones, then merges every branch sequentially, then pushes once.
		// The first merge fast-forwards; the rest produce real merge commits,
		// which generates multiple commits for "git rev-list --ancestry-path --merges ...".
		dstPath := t.TempDir()
		u, _ := url.Parse(giteaURL.String())
		u.Path = fmt.Sprintf("%s/%s.git", user2.Name, repoName)
		u.User = url.UserPassword(user1.Name, userPassword)

		doGitClone(dstPath, u)(t)

		// Capture each branch's expected merge commit hash from the local clone,
		// so we can assert that Gitea recorded the correct merge commit per PR
		// (and not just "some merge commit" — see the regression where every PR
		// was attributed to the last merge in the push).
		expectedMergeCommits := make([]string, len(branchNames))
		for i, branchName := range branchNames {
			doGitMerge(dstPath, "--no-ff", "-m", "merge "+branchName, "origin/"+branchName)(t)
			head, _, cmdErr := gitcmd.NewCommand("rev-parse", "HEAD").WithDir(dstPath).RunStdString(t.Context())
			require.NoError(t, cmdErr)
			expectedMergeCommits[i] = strings.TrimSpace(head)
		}
		doGitPushTestRepository(dstPath, "origin", defaultBranch)(t)

		// Every PR should be detected as manually merged by the background task, not just the last one.
		require.Eventually(t, func() bool {
			for _, apiPull := range apiPulls {
				pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
				if !pr.HasMerged {
					return false
				}
			}
			return true
		}, 10*time.Second, 100*time.Millisecond)

		for i, apiPull := range apiPulls {
			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
			assert.Truef(t, pr.HasMerged, "PR %d (%s) should be marked as merged", i+1, branchNames[i])
			assert.Equalf(t, issues_model.PullRequestStatusManuallyMerged, pr.Status, "PR %d (%s) should have ManuallyMerged status", i+1, branchNames[i])
			assert.Equalf(t, user1.ID, pr.MergerID, "PR %d (%s) merger should be the pusher", i+1, branchNames[i])
			assert.Equalf(t, expectedMergeCommits[i], pr.MergedCommitID, "PR %d (%s) should be attributed to its own merge commit, not another PR's", i+1, branchNames[i])
		}
	})
}
