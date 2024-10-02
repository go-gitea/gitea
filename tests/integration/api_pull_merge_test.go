// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	api "code.gitea.io/gitea/modules/structs"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"

	"github.com/stretchr/testify/assert"
)

func TestAPIPullAutoMergeAfterCommitStatusSucceed(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// create a pull request
		session := loginUser(t, "user1")
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		forkedName := "repo1-1"
		testRepoFork(t, session, "user2", "repo1", "user1", forkedName, "")
		defer func() {
			testDeleteRepository(t, session, "user1", forkedName)
		}()
		testEditFile(t, session, "user1", forkedName, "master", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", forkedName, false, "master", "master", "Indexer notifier test pull")

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
		forkedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: forkedName})
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			BaseRepoID: baseRepo.ID,
			BaseBranch: "master",
			HeadRepoID: forkedRepo.ID,
			HeadBranch: "master",
		})

		// add protected branch for commit status
		csrf := GetCSRF(t, session, "/user2/repo1/settings/branches")
		// Change master branch to protected
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings/branches/edit", map[string]string{
			"_csrf":                 csrf,
			"rule_name":             "master",
			"enable_push":           "true",
			"enable_status_check":   "true",
			"status_check_contexts": "gitea/actions",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// add automerge for this repo
		req = NewRequestWithBody(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", baseRepo.OwnerName, baseRepo.Name, pr.Index),
			strings.NewReader(url.Values{
				"do":                        []string{"merge"},
				"merge_when_checks_succeed": []string{"true"},
			}.Encode())).
			AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
		MakeRequest(t, req, http.StatusCreated)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.False(t, pr.HasMerged)
		assert.Empty(t, pr.MergedCommitID)

		// update commit status to success, then it should be merged automatically
		baseGitRepo, err := gitrepo.OpenRepository(db.DefaultContext, baseRepo)
		assert.NoError(t, err)
		sha, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
		assert.NoError(t, err)
		masterCommitID, err := baseGitRepo.GetBranchCommitID("master")
		assert.NoError(t, err)

		branches, _, err := baseGitRepo.GetBranchNames(0, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"sub-home-md-img-check", "home-md-img-check", "pr-to-update", "branch2", "DefaultBranch", "develop", "feature/1", "master"}, branches)
		baseGitRepo.Close()
		defer func() {
			testResetRepo(t, baseRepo.RepoPath(), "master", masterCommitID)
		}()

		err = commitstatus_service.CreateCommitStatus(db.DefaultContext, baseRepo, user1, sha, &git_model.CommitStatus{
			State:     api.CommitStatusSuccess,
			TargetURL: "https://gitea.com",
			Context:   "gitea/actions",
		})
		assert.NoError(t, err)

		time.Sleep(2 * time.Second)

		// reload pr again
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
		assert.True(t, pr.HasMerged)
		assert.NotEmpty(t, pr.MergedCommitID)

		unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
	})
}
