// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/routers/common"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/codesearch"
	"gitea.dev/services/context"
)

const tplUserCode templates.TplName = "user/code"

func CodeSearch(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled {
		ctx.Redirect(ctx.ContextUser.HomeLink())
		return
	}
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}

	ctx.Data["IsPackageEnabled"] = setting.Packages.Enabled
	ctx.Data["Title"] = ctx.Tr("explore.code")
	ctx.Data["IsCodePage"] = true

	common.SearchCode(ctx, &codesearch.Options{Engine: codesearch.Indexer, OwnerName: ctx.ContextUser.Name})
	if !ctx.Written() {
		ctx.HTML(http.StatusOK, tplUserCode)
	}
}
