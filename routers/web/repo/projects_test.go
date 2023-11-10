// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestCheckProjectBoardChangePermissions(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1/projects/1/2")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	ctx.SetParams(":id", "1")
	ctx.SetParams(":boardID", "2")

	project, board := checkProjectBoardChangePermissions(ctx)
	assert.NotNil(t, project)
	assert.NotNil(t, board)
	assert.False(t, ctx.Written())
}
