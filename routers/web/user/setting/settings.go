// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strconv"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
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
	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.HTTPError(http.StatusBadRequest, "json decode failed")
		return
	}
	_ = user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyCodeViewShowFileTree, strconv.FormatBool(form.CodeViewShowFileTree))
	ctx.JSONOK()
}
