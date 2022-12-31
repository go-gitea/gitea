// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/routers/web/org"

	"github.com/stretchr/testify/assert"
)

func TestCheckProjectBoardChangePermissions(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/-/projects/1/2")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	ctx.SetParams(":id", "1")
	ctx.SetParams(":boardID", "2")
	ctx.ContextUser = ctx.Doer // user2

	project, board := org.CheckProjectBoardChangePermissions(ctx)
	assert.NotNil(t, project)
	assert.NotNil(t, board)
	assert.False(t, ctx.Written())
}
