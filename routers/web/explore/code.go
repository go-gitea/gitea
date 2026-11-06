// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"net/http"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/routers/common"
	"gitea.dev/services/codesearch"
	"gitea.dev/services/context"
)

const tplExploreCode templates.TplName = "explore/code"

func Code(ctx *context.Context) {
	if !setting.Indexer.RepoIndexerEnabled || setting.Service.Explore.DisableCodePage {
		ctx.Redirect(setting.AppSubURL + "/explore")
		return
	}

	ctx.Data["UsersPageIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["OrganizationsPageIsDisabled"] = setting.Service.Explore.DisableOrganizationsPage
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["Title"] = ctx.Tr("explore_title")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreCode"] = true
	ctx.Data["PageIsViewCode"] = true

	common.SearchCode(ctx, &codesearch.Options{Engine: codesearch.Indexer})
	if !ctx.Written() {
		ctx.HTML(http.StatusOK, tplExploreCode)
	}
}
