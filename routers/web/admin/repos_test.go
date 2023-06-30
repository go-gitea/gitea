// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestUpdateRepoPost(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := test.MockContext(t, "admin/repos")
	test.LoadUser(t, ctx, 1)

	ctx.Req.Form.Set("enable_size_limit", "on")
	ctx.Req.Form.Set("repo_size_limit", "222 kcmcm")

	UpdateRepoPost(ctx)

	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}
