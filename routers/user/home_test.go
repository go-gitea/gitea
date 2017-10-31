// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"

	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
)

func TestIssues(t *testing.T) {
	setting.UI.IssuePagingNum = 1
	assert.NoError(t, models.LoadFixtures())

	ctx := test.MockContext(t)
	ctx.User = models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	ctx.SetParams(":type", "issues")
	ctx.Req.Form.Set("state", "closed")
	Issues(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())

	assert.EqualValues(t, map[int64]int64{1: 1, 2: 1}, ctx.Data["Counts"])
	assert.EqualValues(t, true, ctx.Data["IsShowClosed"])
	assert.Len(t, ctx.Data["Issues"], 1)
	assert.Len(t, ctx.Data["Repos"], 2)
}
