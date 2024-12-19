// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/private"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestHandlePullRequestMerging(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr, err := issues_model.GetUnmergedPullRequest(db.DefaultContext, 1, 1, "branch2", "master", issues_model.PullRequestFlowGithub)
	assert.NoError(t, err)
	assert.NoError(t, pr.LoadBaseRepo(db.DefaultContext))

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	err = pull_model.ScheduleAutoMerge(db.DefaultContext, user1, pr.ID, repo_model.MergeStyleSquash, "squash merge a pr")
	assert.NoError(t, err)

	autoMerge := unittest.AssertExistsAndLoadBean(t, &pull_model.AutoMerge{PullID: pr.ID})

	ctx, resp := contexttest.MockPrivateContext(t, "/")
	handlePullRequestMerging(ctx, &private.HookOptions{
		PullRequestID: pr.ID,
		UserID:        2,
	}, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, []*repo_module.PushUpdateOptions{
		{NewCommitID: "01234567"},
	})
	assert.Empty(t, resp.Body.String())
	pr, err = issues_model.GetPullRequestByID(db.DefaultContext, pr.ID)
	assert.NoError(t, err)
	assert.True(t, pr.HasMerged)
	assert.EqualValues(t, "01234567", pr.MergedCommitID)

	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{ID: autoMerge.ID})
}
