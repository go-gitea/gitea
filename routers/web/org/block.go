// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
)

const (
	tplSettingsBlockedUsers base.TplName = "org/settings/blocked_users"
)

func BlockedUsers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("user.block.list")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsBlockedUsers"] = true

	shared_user.BlockedUsers(ctx, ctx.ContextUser)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsBlockedUsers)
}

func BlockedUsersPost(ctx *context.Context) {
	shared_user.BlockedUsersPost(ctx, ctx.ContextUser)
	if ctx.Written() {
		return
	}

	ctx.Redirect(ctx.ContextUser.OrganisationLink() + "/settings/blocked_users")
}
