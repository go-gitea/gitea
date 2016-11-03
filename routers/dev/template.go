// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dev

import (
	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/base"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/go-gitea/gitea/modules/setting"
)

func TemplatePreview(ctx *context.Context) {
	ctx.Data["User"] = models.User{Name: "Unknown"}
	ctx.Data["AppName"] = setting.AppName
	ctx.Data["AppVer"] = setting.AppVer
	ctx.Data["AppUrl"] = setting.AppUrl
	ctx.Data["Code"] = "2014031910370000009fff6782aadb2162b4a997acb69d4400888e0b9274657374"
	ctx.Data["ActiveCodeLives"] = setting.Service.ActiveCodeLives / 60
	ctx.Data["ResetPwdCodeLives"] = setting.Service.ResetPwdCodeLives / 60
	ctx.Data["CurDbValue"] = ""

	ctx.HTML(200, base.TplName(ctx.Params("*")))
}
