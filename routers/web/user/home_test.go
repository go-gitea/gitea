// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestArchivedIssues(t *testing.T) {
	// Arrange
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := test.MockContext(t, "issues")
	test.LoadUser(t, ctx, 30)
	ctx.Req.Form.Set("state", "open")

	// Assume: User 30 has access to two Repos with Issues, one of the Repos being archived.
	repos, _, _ := repo_model.GetUserRepositories(&repo_model.SearchRepoOptions{Actor: ctx.Doer})
	assert.Len(t, repos, 3)
	IsArchived := make(map[int64]bool)
	NumIssues := make(map[int64]int)
	for _, repo := range repos {
		IsArchived[repo.ID] = repo.IsArchived
		NumIssues[repo.ID] = repo.NumIssues
	}
	assert.False(t, IsArchived[50])
	assert.EqualValues(t, 1, NumIssues[50])
	assert.True(t, IsArchived[51])
	assert.EqualValues(t, 1, NumIssues[51])

	// Act
	Issues(ctx)

	// Assert: One Issue (ID 30) from one Repo (ID 50) is retrieved, while nothing from archived Repo 51 is retrieved
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())

	assert.EqualValues(t, map[int64]int64{50: 1}, ctx.Data["Counts"])
	assert.Len(t, ctx.Data["Issues"], 1)
	assert.Len(t, ctx.Data["Repos"], 1)
}

func TestIssues(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := test.MockContext(t, "issues")
	test.LoadUser(t, ctx, 2)
	ctx.Req.Form.Set("state", "closed")
	Issues(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())

	assert.EqualValues(t, map[int64]int64{1: 1, 2: 1}, ctx.Data["Counts"])
	assert.EqualValues(t, true, ctx.Data["IsShowClosed"])
	assert.Len(t, ctx.Data["Issues"], 1)
	assert.Len(t, ctx.Data["Repos"], 2)
}

func TestPulls(t *testing.T) {
	setting.UI.IssuePagingNum = 20
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := test.MockContext(t, "pulls")
	test.LoadUser(t, ctx, 2)
	ctx.Req.Form.Set("state", "open")
	Pulls(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())

	assert.Len(t, ctx.Data["Issues"], 4)
}

func TestMilestones(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := test.MockContext(t, "milestones")
	test.LoadUser(t, ctx, 2)
	ctx.SetParams("sort", "issues")
	ctx.Req.Form.Set("state", "closed")
	ctx.Req.Form.Set("sort", "furthestduedate")
	Milestones(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assert.EqualValues(t, map[int64]int64{1: 1}, ctx.Data["Counts"])
	assert.EqualValues(t, true, ctx.Data["IsShowClosed"])
	assert.EqualValues(t, "furthestduedate", ctx.Data["SortType"])
	assert.EqualValues(t, 1, ctx.Data["Total"])
	assert.Len(t, ctx.Data["Milestones"], 1)
	assert.Len(t, ctx.Data["Repos"], 2) // both repo 42 and 1 have milestones and both are owned by user 2
}

func TestMilestonesForSpecificRepo(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := test.MockContext(t, "milestones")
	test.LoadUser(t, ctx, 2)
	ctx.SetParams("sort", "issues")
	ctx.SetParams("repo", "1")
	ctx.Req.Form.Set("state", "closed")
	ctx.Req.Form.Set("sort", "furthestduedate")
	Milestones(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assert.EqualValues(t, map[int64]int64{1: 1}, ctx.Data["Counts"])
	assert.EqualValues(t, true, ctx.Data["IsShowClosed"])
	assert.EqualValues(t, "furthestduedate", ctx.Data["SortType"])
	assert.EqualValues(t, 1, ctx.Data["Total"])
	assert.Len(t, ctx.Data["Milestones"], 1)
	assert.Len(t, ctx.Data["Repos"], 2) // both repo 42 and 1 have milestones and both are owned by user 2
}
