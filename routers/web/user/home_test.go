// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestArchivedIssues(t *testing.T) {
	// Arrange
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := contexttest.MockContext(t, "issues")
	contexttest.LoadUser(t, ctx, 30)
	ctx.Req.Form.Set("state", "open")

	// Assume: User 30 has access to two Repos with Issues, one of the Repos being archived.
	repos, _, _ := repo_model.GetUserRepositories(db.DefaultContext, &repo_model.SearchRepoOptions{Actor: ctx.Doer})
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
	assert.EqualValues(t, http.StatusOK, ctx.Resp.WrittenStatus())

	assert.Len(t, ctx.Data["Issues"], 1)
}

func TestIssues(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := contexttest.MockContext(t, "issues")
	contexttest.LoadUser(t, ctx, 2)
	ctx.Req.Form.Set("state", "closed")
	Issues(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.WrittenStatus())

	assert.EqualValues(t, true, ctx.Data["IsShowClosed"])
	assert.Len(t, ctx.Data["Issues"], 1)
}

func TestPulls(t *testing.T) {
	setting.UI.IssuePagingNum = 20
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := contexttest.MockContext(t, "pulls")
	contexttest.LoadUser(t, ctx, 2)
	ctx.Req.Form.Set("state", "open")
	Pulls(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.WrittenStatus())

	assert.Len(t, ctx.Data["Issues"], 5)
}

func TestMilestones(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, unittest.LoadFixtures())

	ctx, _ := contexttest.MockContext(t, "milestones")
	contexttest.LoadUser(t, ctx, 2)
	ctx.SetPathParam("sort", "issues")
	ctx.Req.Form.Set("state", "closed")
	ctx.Req.Form.Set("sort", "furthestduedate")
	Milestones(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.WrittenStatus())
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

	ctx, _ := contexttest.MockContext(t, "milestones")
	contexttest.LoadUser(t, ctx, 2)
	ctx.SetPathParam("sort", "issues")
	ctx.SetPathParam("repo", "1")
	ctx.Req.Form.Set("state", "closed")
	ctx.Req.Form.Set("sort", "furthestduedate")
	Milestones(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.WrittenStatus())
	assert.EqualValues(t, map[int64]int64{1: 1}, ctx.Data["Counts"])
	assert.EqualValues(t, true, ctx.Data["IsShowClosed"])
	assert.EqualValues(t, "furthestduedate", ctx.Data["SortType"])
	assert.EqualValues(t, 1, ctx.Data["Total"])
	assert.Len(t, ctx.Data["Milestones"], 1)
	assert.Len(t, ctx.Data["Repos"], 2) // both repo 42 and 1 have milestones and both are owned by user 2
}

func TestDashboardPagination(t *testing.T) {
	ctx, _ := contexttest.MockContext(t, "/", contexttest.MockContextOption{Render: templates.HTMLRenderer()})
	page := context.NewPagination(10, 3, 1, 3)

	setting.AppSubURL = "/SubPath"
	out, err := ctx.RenderToHTML("base/paginate", map[string]any{"Link": setting.AppSubURL, "Page": page})
	assert.NoError(t, err)
	assert.Contains(t, out, `<a class=" item navigation" href="/SubPath/?page=2">`)

	setting.AppSubURL = ""
	out, err = ctx.RenderToHTML("base/paginate", map[string]any{"Link": setting.AppSubURL, "Page": page})
	assert.NoError(t, err)
	assert.Contains(t, out, `<a class=" item navigation" href="/?page=2">`)
}
