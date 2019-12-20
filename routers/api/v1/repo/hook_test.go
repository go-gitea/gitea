// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestTestHook(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/wiki/_pages")
	ctx.SetParams(":id", "1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	TestHook(&context.APIContext{Context: ctx, Org: nil})
	assert.EqualValues(t, http.StatusNoContent, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.HookTask{
		RepoID: 1,
		HookID: 1,
	}, models.Cond("is_delivered=?", false))
}
