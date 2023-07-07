// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
	"code.gitea.io/gitea/services/user"
)

const (
	tplSettingsAccount base.TplName = "user/settings/account"
)

// Account renders change user's password, user's email and user suicide page
func Account(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.account")
	ctx.Data["PageIsSettingsAccount"] = true
	ctx.Data["Email"] = ctx.Doer.Email
	ctx.Data["EnableNotifyMail"] = setting.Service.EnableNotifyMail

	loadAccountData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsAccount)
}

// AccountPost response for change user's password
func AccountPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ChangePasswordForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true

	if ctx.HasError() {
		loadAccountData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsAccount)
		return
	}

	if len(form.Password) < setting.MinPasswordLength {
		ctx.Flash.Error(ctx.Tr("auth.password_too_short", setting.MinPasswordLength))
	} else if ctx.Doer.IsPasswordSet() && !ctx.Doer.ValidatePassword(form.OldPassword) {
		ctx.Flash.Error(ctx.Tr("settings.password_incorrect"))
	} else if form.Password != form.Retype {
		ctx.Flash.Error(ctx.Tr("form.password_not_match"))
	} else if !password.IsComplexEnough(form.Password) {
		ctx.Flash.Error(password.BuildComplexityError(ctx.Locale))
	} else if pwned, err := password.IsPwned(ctx, form.Password); pwned || err != nil {
		errMsg := ctx.Tr("auth.password_pwned")
		if err != nil {
			log.Error(err.Error())
			errMsg = ctx.Tr("auth.password_pwned_err")
		}
		ctx.Flash.Error(errMsg)
	} else {
		var err error
		if err = ctx.Doer.SetPassword(form.Password); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
		if err := user_model.UpdateUserCols(ctx, ctx.Doer, "salt", "passwd_hash_algo", "passwd"); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
		log.Trace("User password updated: %s", ctx.Doer.Name)
		ctx.Flash.Success(ctx.Tr("settings.change_password_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}

// EmailPost response for change user's email
func EmailPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddEmailForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true

	// Make emailaddress primary.
	if ctx.FormString("_method") == "PRIMARY" {
		if err := user_model.MakeEmailPrimary(&user_model.EmailAddress{ID: ctx.FormInt64("id")}); err != nil {
			ctx.ServerError("MakeEmailPrimary", err)
			return
		}

		log.Trace("Email made primary: %s", ctx.Doer.Name)
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}
	// Send activation Email
	if ctx.FormString("_method") == "SENDACTIVATION" {
		var address string
		if setting.CacheService.Enabled && ctx.Cache.IsExist("MailResendLimit_"+ctx.Doer.LowerName) {
			log.Error("Send activation: activation still pending")
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}

		id := ctx.FormInt64("id")
		email, err := user_model.GetEmailAddressByID(ctx.Doer.ID, id)
		if err != nil {
			log.Error("GetEmailAddressByID(%d,%d) error: %v", ctx.Doer.ID, id, err)
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}
		if email == nil {
			log.Warn("Send activation failed: EmailAddress[%d] not found for user: %-v", id, ctx.Doer)
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}
		if email.IsActivated {
			log.Debug("Send activation failed: email %s is already activated for user: %-v", email.Email, ctx.Doer)
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}
		if email.IsPrimary {
			if ctx.Doer.IsActive && !setting.Service.RegisterEmailConfirm {
				log.Debug("Send activation failed: email %s is already activated for user: %-v", email.Email, ctx.Doer)
				ctx.Redirect(setting.AppSubURL + "/user/settings/account")
				return
			}
			// Only fired when the primary email is inactive (Wrong state)
			mailer.SendActivateAccountMail(ctx.Locale, ctx.Doer)
		} else {
			mailer.SendActivateEmailMail(ctx.Doer, email)
		}
		address = email.Email

		if setting.CacheService.Enabled {
			if err := ctx.Cache.Put("MailResendLimit_"+ctx.Doer.LowerName, ctx.Doer.LowerName, 180); err != nil {
				log.Error("Set cache(MailResendLimit) fail: %v", err)
			}
		}
		ctx.Flash.Info(ctx.Tr("settings.add_email_confirmation_sent", address, timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale)))
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}
	// Set Email Notification Preference
	if ctx.FormString("_method") == "NOTIFICATION" {
		preference := ctx.FormString("preference")
		if !(preference == user_model.EmailNotificationsEnabled ||
			preference == user_model.EmailNotificationsOnMention ||
			preference == user_model.EmailNotificationsDisabled ||
			preference == user_model.EmailNotificationsAndYourOwn) {
			log.Error("Email notifications preference change returned unrecognized option %s: %s", preference, ctx.Doer.Name)
			ctx.ServerError("SetEmailPreference", errors.New("option unrecognized"))
			return
		}
		if err := user_model.SetEmailNotifications(ctx.Doer, preference); err != nil {
			log.Error("Set Email Notifications failed: %v", err)
			ctx.ServerError("SetEmailNotifications", err)
			return
		}
		log.Trace("Email notifications preference made %s: %s", preference, ctx.Doer.Name)
		ctx.Flash.Success(ctx.Tr("settings.email_preference_set_success"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}

	if ctx.HasError() {
		loadAccountData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsAccount)
		return
	}

	email := &user_model.EmailAddress{
		UID:         ctx.Doer.ID,
		Email:       form.Email,
		IsActivated: !setting.Service.RegisterEmailConfirm,
	}
	if err := user_model.AddEmailAddress(ctx, email); err != nil {
		if user_model.IsErrEmailAlreadyUsed(err) {
			loadAccountData(ctx)

			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplSettingsAccount, &form)
			return
		} else if user_model.IsErrEmailCharIsNotSupported(err) ||
			user_model.IsErrEmailInvalid(err) {
			loadAccountData(ctx)

			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tplSettingsAccount, &form)
			return
		}
		ctx.ServerError("AddEmailAddress", err)
		return
	}

	// Send confirmation email
	if setting.Service.RegisterEmailConfirm {
		mailer.SendActivateEmailMail(ctx.Doer, email)
		if setting.CacheService.Enabled {
			if err := ctx.Cache.Put("MailResendLimit_"+ctx.Doer.LowerName, ctx.Doer.LowerName, 180); err != nil {
				log.Error("Set cache(MailResendLimit) fail: %v", err)
			}
		}
		ctx.Flash.Info(ctx.Tr("settings.add_email_confirmation_sent", email.Email, timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale)))
	} else {
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))
	}

	log.Trace("Email address added: %s", email.Email)
	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}

// DeleteEmail response for delete user's email
func DeleteEmail(ctx *context.Context) {
	if err := user_model.DeleteEmailAddress(&user_model.EmailAddress{ID: ctx.FormInt64("id"), UID: ctx.Doer.ID}); err != nil {
		ctx.ServerError("DeleteEmail", err)
		return
	}
	log.Trace("Email address deleted: %s", ctx.Doer.Name)

	ctx.Flash.Success(ctx.Tr("settings.email_deletion_success"))
	ctx.JSON(http.StatusOK, map[string]any{
		"redirect": setting.AppSubURL + "/user/settings/account",
	})
}

// DeleteAccount render user suicide page and response for delete user himself
func DeleteAccount(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true

	if _, _, err := auth.UserSignIn(ctx.Doer.Name, ctx.FormString("password")); err != nil {
		if user_model.IsErrUserNotExist(err) {
			loadAccountData(ctx)

			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_password"), tplSettingsAccount, nil)
		} else {
			ctx.ServerError("UserSignIn", err)
		}
		return
	}

	if err := user.DeleteUser(ctx, ctx.Doer, false); err != nil {
		switch {
		case models.IsErrUserOwnRepos(err):
			ctx.Flash.Error(ctx.Tr("form.still_own_repo"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		case models.IsErrUserHasOrgs(err):
			ctx.Flash.Error(ctx.Tr("form.still_has_org"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		case models.IsErrUserOwnPackages(err):
			ctx.Flash.Error(ctx.Tr("form.still_own_packages"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		default:
			ctx.ServerError("DeleteUser", err)
		}
	} else {
		log.Trace("Account deleted: %s", ctx.Doer.Name)
		ctx.Redirect(setting.AppSubURL + "/")
	}
}

func loadAccountData(ctx *context.Context) {
	emlist, err := user_model.GetEmailAddresses(ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetEmailAddresses", err)
		return
	}
	type UserEmail struct {
		user_model.EmailAddress
		CanBePrimary bool
	}
	pendingActivation := setting.CacheService.Enabled && ctx.Cache.IsExist("MailResendLimit_"+ctx.Doer.LowerName)
	emails := make([]*UserEmail, len(emlist))
	for i, em := range emlist {
		var email UserEmail
		email.EmailAddress = *em
		email.CanBePrimary = em.IsActivated
		emails[i] = &email
	}
	ctx.Data["Emails"] = emails
	ctx.Data["EmailNotificationsPreference"] = ctx.Doer.EmailNotifications()
	ctx.Data["ActivationsPending"] = pendingActivation
	ctx.Data["CanAddEmails"] = !pendingActivation || !setting.Service.RegisterEmailConfirm

	if setting.Service.UserDeleteWithCommentsMaxTime != 0 {
		ctx.Data["UserDeleteWithCommentsMaxTime"] = setting.Service.UserDeleteWithCommentsMaxTime.String()
		ctx.Data["UserDeleteWithComments"] = ctx.Doer.CreatedUnix.AsTime().Add(setting.Service.UserDeleteWithCommentsMaxTime).After(time.Now())
	}
}
