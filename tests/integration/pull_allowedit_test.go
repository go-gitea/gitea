// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
)

func TestPullAllowMaintainerEdit(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// create a pull request
		session := loginUser(t, "user1")
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		forkedName := "repo1"
		testRepoFork(t, session, "org3", "repo5", "user1", forkedName, "master")
		defer func() {
			testDeleteRepository(t, session, "user1", forkedName)
		}()
		testEditFile(t, session, "user1", forkedName, "master", "README.md", "Hello, World (Edited)\n")
		testPullCreate(t, session, "user1", forkedName, false, "master", "master", "Indexer notifier test pull")

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "org3", Name: "repo5"})
		forkedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: forkedName})
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{
			BaseRepoID: baseRepo.ID,
			BaseBranch: "master",
			HeadRepoID: forkedRepo.ID,
			HeadBranch: "master",
		})
		assert.False(t, pr.AllowMaintainerEdit)
		assert.NoError(t, pr.LoadIssue(db.DefaultContext))

		// allow org3's member to edit the branch's files
		err := pull_service.SetAllowEdits(db.DefaultContext, user1, pr, true)
		assert.NoError(t, err)

		// user2 is in org3 team
		session = loginUser(t, "user2")
		testEditFile(t, session, "user1", forkedName, "master", "README.md", "Hello, World (Edited)\n")
	})
}
