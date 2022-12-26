// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCheckBoardColumnChangePermissions(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/boards/1/2")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	ctx.SetParams(":id", "1")
	ctx.SetParams(":boardID", "2")

	board, column := checkBoardColumnChangePermissions(ctx)
	assert.NotNil(t, board)
	assert.NotNil(t, column)
	assert.False(t, ctx.Written())
}
