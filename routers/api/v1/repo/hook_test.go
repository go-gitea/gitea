// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestTestHook(t *testing.T) {
	unittest.PrepareTestEnv(t)

	hook := &webhook.Webhook{
		RepoID:      1,
		URL:         "https://www.example.com/test_hook",
		ContentType: webhook.ContentTypeJSON,
		Events:      `{"push_only":true}`,
		IsActive:    true,
	}
	assert.NoError(t, db.Insert(t.Context(), hook))

	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1/wiki/_pages")
	ctx.SetPathParam("id", strconv.FormatInt(hook.ID, 10))
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	TestHook(ctx)
	assert.Equal(t, http.StatusNoContent, ctx.Resp.WrittenStatus())

	unittest.AssertExistsAndLoadBean(t, &webhook.HookTask{
		HookID: hook.ID,
	}, unittest.Cond("is_delivered=?", false))
}
