// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org_test

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/routers/web/org"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestChangeProjectStatusRejectsForeignProjects(t *testing.T) {
	unittest.PrepareTestEnv(t)
	// project 4 is owned by user2 not user1
	ctx, _ := contexttest.MockContext(t, "user1/-/projects/4/close")
	contexttest.LoadUser(t, ctx, 1)
	ctx.ContextUser = ctx.Doer
	ctx.SetPathParam("action", "close")
	ctx.SetPathParam("id", "4")

	org.ChangeProjectStatus(ctx)

	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}
