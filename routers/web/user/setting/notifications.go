// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
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

	fineGrainedPreference, err := user_model.GetUserNotificationSettings(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetUserNotificationSettings", err)
		return
	}
	ctx.Data["ActionsEmailNotificationsPreference"] = fineGrainedPreference.Actions

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
		log.Error("Email notifications preference change returned unrecognized option %s: %s", preference, ctx.Doer.Name)
		ctx.ServerError("NotificationsEmailPost", errors.New("option unrecognized"))
		return
	}
	opts := &user.UpdateOptions{
		EmailNotificationsPreference: optional.Some(preference),
	}
	if err := user.UpdateUser(ctx, ctx.Doer, opts); err != nil {
		log.Error("Set Email Notifications failed: %v", err)
		ctx.ServerError("UpdateUser", err)
		return
	}
	log.Trace("Email notifications preference made %s: %s", preference, ctx.Doer.Name)
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
	if !(preference == user_model.NotificationGiteaActionsAll ||
		preference == user_model.NotificationGiteaActionsDisabled ||
		preference == user_model.NotificationGiteaActionsFailureOnly) {
		log.Error("Actions Email notifications preference change returned unrecognized option %s: %s", preference, ctx.Doer.Name)
		ctx.ServerError("NotificationsActionsEmailPost", errors.New("option unrecognized"))
		return
	}
	opts := &user.UpdateNotificationSettingsOptions{
		Actions: optional.Some(preference),
	}
	if err := user.UpdateNotificationSettings(ctx, ctx.Doer.ID, opts); err != nil {
		log.Error("Cannot set actions email notifications preference: %v", err)
		ctx.ServerError("UpdateNotificationSettings", err)
		return
	}
	log.Trace("Actions email notifications preference made %s: %s", preference, ctx.Doer.Name)
	ctx.Flash.Success(ctx.Tr("settings.email_preference_set_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/notifications")
}
