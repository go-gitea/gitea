// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"

	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/user"
)

const tplSettingsNotifications templates.TplName = "user/settings/notifications"

// Notifications render user's notifications settings
func Notifications(ctx *context.Context) {
	if !setting.Service.EnableNotifyMail {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Title"] = ctx.Tr("notifications")
	ctx.Data["PageIsSettingsNotifications"] = true
	ctx.Data["EmailNotificationsPreference"] = ctx.Doer.EmailNotificationsPreference

	actionsEmailPref, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyEmailNotificationGiteaActions, user_model.SettingEmailNotificationGiteaActionsFailureOnly)
	if err != nil {
		ctx.ServerError("GetUserSetting", err)
		return
	}
	ctx.Data["ActionsEmailNotificationsPreference"] = actionsEmailPref

	ctx.HTML(http.StatusOK, tplSettingsNotifications)
}

// NotificationsEmailPost set user's email notification preference
func NotificationsEmailPost(ctx *context.Context) {
	if !setting.Service.EnableNotifyMail {
		ctx.NotFound(nil)
		return
	}

	preference := ctx.FormString("preference")
	if !(preference == user_model.EmailNotificationsEnabled ||
		preference == user_model.EmailNotificationsOnMention ||
		preference == user_model.EmailNotificationsDisabled ||
		preference == user_model.EmailNotificationsAndYourOwn) {
		ctx.Flash.Error(ctx.Tr("invalid_data", preference))
		ctx.Redirect(setting.AppSubURL + "/user/settings/notifications")
		return
	}
	opts := &user.UpdateOptions{
		EmailNotificationsPreference: optional.Some(preference),
	}
	if err := user.UpdateUser(ctx, ctx.Doer, opts); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.email_preference_set_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/notifications")
}

// NotificationsActionsEmailPost set user's email notification preference on Gitea Actions
func NotificationsActionsEmailPost(ctx *context.Context) {
	if !setting.Actions.Enabled || unit.TypeActions.UnitGlobalDisabled() {
		ctx.NotFound(nil)
		return
	}

	preference := ctx.FormString("preference")
	if !(preference == user_model.SettingEmailNotificationGiteaActionsAll ||
		preference == user_model.SettingEmailNotificationGiteaActionsDisabled ||
		preference == user_model.SettingEmailNotificationGiteaActionsFailureOnly) {
		ctx.Flash.Error(ctx.Tr("invalid_data", preference))
		ctx.Redirect(setting.AppSubURL + "/user/settings/notifications")
		return
	}
	if err := user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyEmailNotificationGiteaActions, preference); err != nil {
		ctx.ServerError("SetUserSetting", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.email_preference_set_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/notifications")
}
