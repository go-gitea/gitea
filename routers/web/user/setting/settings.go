// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strconv"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
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
		CodeViewShowFileTree *bool   `json:"codeViewShowFileTree"`
		DashboardTab         *string `json:"dashboardTab"`
	}
	form := &preferencesForm{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(form); err != nil {
		ctx.HTTPError(http.StatusBadRequest, "json decode failed")
		return
	}
	if form.CodeViewShowFileTree != nil {
		if err := user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyCodeViewShowFileTree, strconv.FormatBool(*form.CodeViewShowFileTree)); err != nil {
			ctx.ServerError("SetUserSetting", err)
			return
		}
	}
	if tab := form.DashboardTab; tab != nil && (*tab == user_model.DashboardTabOverview || *tab == user_model.DashboardTabActivity) {
		if err := user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyDashboardTab, *tab); err != nil {
			ctx.ServerError("SetUserSetting", err)
			return
		}
	}
	ctx.JSONOK()
}
