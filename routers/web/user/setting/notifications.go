// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/user"
)

const (
	tplSettingsNotifications base.TplName = "user/settings/notifications"
)

// Notifications render manage access token page
func Notifications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.notifications")
	ctx.Data["PageIsSettingsNotifications"] = true
	ctx.Data["Email"] = ctx.Doer.Email
	ctx.Data["EnableNotifyMail"] = setting.Service.EnableNotifyMail

	loadNotificationsData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsNotifications)
}

// NotificationPost response for change user's notification preferences
func NotificationPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsNotifications"] = true

	// Set Email Notification Preference
	if ctx.FormString("_method") == "EMAIL" {
		preference := ctx.FormString("preference")
		if !(preference == user_model.NotificationsEnabled ||
			preference == user_model.NotificationsOnMention ||
			preference == user_model.NotificationsDisabled ||
			preference == user_model.NotificationsAndYourOwn) {
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
		return
		// Set UI Notification Preference
	} else if ctx.FormString("_method") == "UI" {
		preference := ctx.FormString("preference")
		if !(preference == user_model.NotificationsEnabled ||
			preference == user_model.NotificationsOnMention ||
			preference == user_model.NotificationsDisabled ||
			preference == user_model.NotificationsAndYourOwn) {
			log.Error("UI notifications preference change returned unrecognized option %s: %s", preference, ctx.Doer.Name)
			ctx.ServerError("SetUIPreference", errors.New("option unrecognized"))
			return
		}
		opts := &user.UpdateOptions{
			UINotificationsPreference: optional.Some(preference),
		}
		if err := user.UpdateUser(ctx, ctx.Doer, opts); err != nil {
			log.Error("Set UI Notifications failed: %v", err)
			ctx.ServerError("UpdateUser", err)
			return
		}
		log.Trace("UI notifications preference made %s: %s", preference, ctx.Doer.Name)
		ctx.Flash.Success(ctx.Tr("settings.ui_preference_set_success"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/notifications")
		return
	}

	if ctx.HasError() {
		loadAccountData(ctx)
		ctx.HTML(http.StatusOK, tplSettingsAccount)
		return
	}
}

func loadNotificationsData(ctx *context.Context) {
	emlist, err := user_model.GetEmailAddresses(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetEmailAddresses", err)
		return
	}
	type UserEmail struct {
		user_model.EmailAddress
	}
	emails := make([]*UserEmail, len(emlist))
	for i, em := range emlist {
		if !em.IsActivated {
			continue
		}
		var email UserEmail
		email.EmailAddress = *em
		emails[i] = &email
	}
	ctx.Data["Emails"] = emails
	ctx.Data["EmailNotificationsPreference"] = ctx.Doer.EmailNotificationsPreference
	ctx.Data["UINotificationsPreference"] = ctx.Doer.UINotificationsPreference
}
