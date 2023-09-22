// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"code.gitea.io/gitea/internal/modules/context"
	"code.gitea.io/gitea/internal/modules/setting"
)

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(setting.AppSubURL + "/admin/actions/runners")
}
