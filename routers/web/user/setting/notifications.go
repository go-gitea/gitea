// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

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
		ctx.ServerError("SetEmailPreference", errors.New("option unrecognized"))
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
