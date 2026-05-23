// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org_test

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/org"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/assert"
)

func TestCheckProjectColumnChangePermissions(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/-/projects/4/4")
	contexttest.LoadUser(t, ctx, 2)
	ctx.ContextUser = ctx.Doer // user2
	ctx.SetPathParam("id", "4")
	ctx.SetPathParam("columnID", "4")

	project, column := org.CheckProjectColumnChangePermissions(ctx)
	assert.NotNil(t, project)
	assert.NotNil(t, column)
	assert.False(t, ctx.Written())
}

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

func TestAddColumnToProjectPostRejectsForeignProjects(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user1/-/projects/4/columns/new")
	contexttest.LoadUser(t, ctx, 1)
	ctx.ContextUser = ctx.Doer
	ctx.SetPathParam("id", "4")
	web.SetForm(ctx, &forms.EditProjectColumnForm{Title: "foreign"})

	org.AddColumnToProjectPost(ctx)

	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}
