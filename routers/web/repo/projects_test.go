// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCheckProjectBoardChangePermissions(t *testing.T) {
	db.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/projects/1/2")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	ctx.SetParams(":id", "1")
	ctx.SetParams(":boardID", "2")

	project, board := checkProjectBoardChangePermissions(ctx)
	assert.NotNil(t, project)
	assert.NotNil(t, board)
	assert.False(t, ctx.Written())
}
