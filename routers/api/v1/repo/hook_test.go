// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestTestHook(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/wiki/_pages")
	ctx.SetPathParam("id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	TestHook(ctx)
	assert.EqualValues(t, http.StatusNoContent, ctx.Resp.WrittenStatus())

	unittest.AssertExistsAndLoadBean(t, &webhook.HookTask{
		HookID: 1,
	}, unittest.Cond("is_delivered=?", false))
}
