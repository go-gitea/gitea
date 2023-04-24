// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSettingsActions base.TplName = "user/settings/actions"
)

func Actions(ctx *context.Context) {
	pageType := ctx.Params(":type")
	if pageType == "secrets" {
		ctx.Data["PageIsSettingsSecrets"] = true
		PrepareSecretsData(ctx)
	} else {
		ctx.ServerError("Unknown Page Type", fmt.Errorf("Unknown Actions Settings Type: %s", pageType))
		return
	}
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageType"] = pageType
	ctx.HTML(http.StatusOK, tplSettingsActions)
}

func RedirectToRunnersSettings(ctx *context.Context) {
	ctx.Redirect(setting.AppSubURL + "/user/settings/actions/secrets")
}
