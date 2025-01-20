// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	tmplRepoActionsGeneral templates.TplName = "repo/settings/actions"
)

// General ...
func General(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsActionsGeneral"] = true
	ctx.Data["Title"] = ctx.Tr("actions.general")
	ctx.Data["PageType"] = "general"

	ctx.Data["Checked"] = 3

	ctx.HTML(http.StatusOK, tmplRepoActionsGeneral)
}

// GeneralPost ...
func GeneralPost(ctx *context.Context) {
	ctx.Data["PageIsSharedSettingsActionsGeneral"] = true
	ctx.Data["Title"] = ctx.Tr("actions.general")
	ctx.Data["PageType"] = "general"

	ctx.Data["Checked"] = 3

	ctx.HTML(http.StatusOK, tmplRepoActionsGeneral)
}
