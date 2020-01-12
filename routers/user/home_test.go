// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestIssues(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, models.LoadFixtures())

	ctx := test.MockContext(t, "issues")
	test.LoadUser(t, ctx, 2)
	ctx.SetParams(":type", "issues")
	ctx.Req.Form.Set("state", "closed")
	Issues(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())

	assert.EqualValues(t, map[int64]int64{1: 1, 2: 1}, ctx.Data["Counts"])
	assert.EqualValues(t, true, ctx.Data["IsShowClosed"])
	assert.Len(t, ctx.Data["Issues"], 1)
	assert.Len(t, ctx.Data["Repos"], 2)
}

func TestMilestones(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, models.LoadFixtures())

	ctx := test.MockContext(t, "milestones")
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
	assert.Len(t, ctx.Data["Repos"], 1)
}

func TestMilestonesForSpecificRepo(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, models.LoadFixtures())

	ctx := test.MockContext(t, "milestones")
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
	assert.Len(t, ctx.Data["Repos"], 1)
}
