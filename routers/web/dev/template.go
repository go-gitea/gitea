// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dev

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

// TemplatePreview render for previewing the indicated template
func TemplatePreview(ctx *context.Context) {
	ctx.Data["User"] = user_model.User{Name: "Unknown"}
	ctx.Data["AppName"] = setting.AppName
	ctx.Data["AppVer"] = setting.AppVer
	ctx.Data["AppUrl"] = setting.AppURL
	ctx.Data["Code"] = "2014031910370000009fff6782aadb2162b4a997acb69d4400888e0b9274657374"
	ctx.Data["ActiveCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale.Language())
	ctx.Data["ResetPwdCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, ctx.Locale.Language())
	ctx.Data["CurDbValue"] = ""

	ctx.HTML(http.StatusOK, base.TplName(ctx.Params("*")))
}
