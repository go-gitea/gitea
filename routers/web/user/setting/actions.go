// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplSettingsActions base.TplName = "user/settings/actions"
)

func Actions(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("actions.actions")
	ctx.Data["PageIsSettingsActions"] = true
	ctx.Data["RunnersBaseLink"] = fmt.Sprintf("%s/runners", ctx.Link)
	ctx.Data["SecretsBaseLink"] = fmt.Sprintf("%s/secrets", ctx.Link)
	prepareSecretsData(ctx)
	ctx.HTML(http.StatusOK, tplSettingsActions)
}
