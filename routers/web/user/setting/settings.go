// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strconv"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/jsonlimit"
	"gitea.dev/modules/setting"
	"gitea.dev/services/context"
)

func SettingsCtxData(ctx *context.Context) {
	ctx.Data["PageIsUserSettings"] = true
	ctx.Data["EnablePackages"] = setting.Packages.Enabled
	ctx.Data["EnableNotifyMail"] = setting.Service.EnableNotifyMail
	ctx.Data["UserDisabledFeatures"] = user_model.DisabledFeaturesWithLoginType(ctx.Doer)
}

func UpdatePreferences(ctx *context.Context) {
	type preferencesForm struct {
		CodeViewShowFileTree bool `json:"codeViewShowFileTree"`
	}
	form := &preferencesForm{}
	if err := jsonlimit.DecodeFromRequest(ctx.Resp, ctx.Req, 10*1024*1024, &form); err != nil {
		ctx.JSONError("invalid request body")
		return
	}
	_ = user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyCodeViewShowFileTree, strconv.FormatBool(form.CodeViewShowFileTree))
	ctx.JSONOK()
}
