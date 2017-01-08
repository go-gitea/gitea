// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/Unknwon/com"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSettingsProfile      base.TplName = "user/settings/profile"
	tplSettingsAvatar       base.TplName = "user/settings/avatar"
	tplSettingsPassword     base.TplName = "user/settings/password"
	tplSettingsEmails       base.TplName = "user/settings/email"
	tplSettingsSSHKeys      base.TplName = "user/settings/sshkeys"
	tplSettingsSocial       base.TplName = "user/settings/social"
	tplSettingsApplications base.TplName = "user/settings/applications"
	tplSettingsDelete       base.TplName = "user/settings/delete"
	tplSecurity             base.TplName = "user/security"
)

// Settings render user's profile page
func Settings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsProfile"] = true
	ctx.HTML(200, tplSettingsProfile)
}

func handleUsernameChange(ctx *context.Context, newName string) {
	// Non-local users are not allowed to change their username.
	if len(newName) == 0 || !ctx.User.IsLocal() {
		return
	}

	// Check if user name has been changed
	if ctx.User.LowerName != strings.ToLower(newName) {
		if err := models.ChangeUserName(ctx.User, newName); err != nil {
			switch {
			case models.IsErrUserAlreadyExist(err):
				ctx.Flash.Error(ctx.Tr("newName_been_taken"))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			case models.IsErrEmailAlreadyUsed(err):
				ctx.Flash.Error(ctx.Tr("form.email_been_used"))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			case models.IsErrNameReserved(err):
				ctx.Flash.Error(ctx.Tr("user.newName_reserved"))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			case models.IsErrNamePatternNotAllowed(err):
				ctx.Flash.Error(ctx.Tr("user.newName_pattern_not_allowed"))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			default:
				ctx.Handle(500, "ChangeUserName", err)
			}
			return
		}
		log.Trace("User name changed: %s -> %s", ctx.User.Name, newName)
	}

	// In case it's just a case change
	ctx.User.Name = newName
	ctx.User.LowerName = strings.ToLower(newName)
}

// SettingsPost response for change user's profile
func SettingsPost(ctx *context.Context, form auth.UpdateProfileForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsProfile"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplSettingsProfile)
		return
	}

	handleUsernameChange(ctx, form.Name)
	if ctx.Written() {
		return
	}

	ctx.User.FullName = form.FullName
	ctx.User.Email = form.Email
	ctx.User.KeepEmailPrivate = form.KeepEmailPrivate
	ctx.User.Website = form.Website
	ctx.User.Location = form.Location
	if err := models.UpdateUser(ctx.User); err != nil {
		ctx.Handle(500, "UpdateUser", err)
		return
	}

	log.Trace("User settings updated: %s", ctx.User.Name)
	ctx.Flash.Success(ctx.Tr("settings.update_profile_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// UpdateAvatarSetting update user's avatar
// FIXME: limit size.
func UpdateAvatarSetting(ctx *context.Context, form auth.AvatarForm, ctxUser *models.User) error {
	ctxUser.UseCustomAvatar = form.Source == auth.AvatarLocal
	if len(form.Gravatar) > 0 {
		ctxUser.Avatar = base.EncodeMD5(form.Gravatar)
		ctxUser.AvatarEmail = form.Gravatar
	}

	if form.Avatar != nil {
		fr, err := form.Avatar.Open()
		if err != nil {
			return fmt.Errorf("Avatar.Open: %v", err)
		}
		defer fr.Close()

		data, err := ioutil.ReadAll(fr)
		if err != nil {
			return fmt.Errorf("ioutil.ReadAll: %v", err)
		}
		if !base.IsImageFile(data) {
			return errors.New(ctx.Tr("settings.uploaded_avatar_not_a_image"))
		}
		if err = ctxUser.UploadAvatar(data); err != nil {
			return fmt.Errorf("UploadAvatar: %v", err)
		}
	} else {
		// No avatar is uploaded but setting has been changed to enable,
		// generate a random one when needed.
		if ctxUser.UseCustomAvatar && !com.IsFile(ctxUser.CustomAvatarPath()) {
			if err := ctxUser.GenerateRandomAvatar(); err != nil {
				log.Error(4, "GenerateRandomAvatar[%d]: %v", ctxUser.ID, err)
			}
		}
	}

	if err := models.UpdateUser(ctxUser); err != nil {
		return fmt.Errorf("UpdateUser: %v", err)
	}

	return nil
}

// SettingsAvatar render user avatar page
func SettingsAvatar(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAvatar"] = true
	ctx.HTML(200, tplSettingsAvatar)
}

// SettingsAvatarPost response for change user's avatar request
func SettingsAvatarPost(ctx *context.Context, form auth.AvatarForm) {
	if err := UpdateAvatarSetting(ctx, form, ctx.User); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.update_avatar_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/avatar")
}

// SettingsDeleteAvatar render delete avatar page
func SettingsDeleteAvatar(ctx *context.Context) {
	if err := ctx.User.DeleteAvatar(); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/avatar")
}

// SettingsPassword render change user's password page
func SettingsPassword(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsPassword"] = true
	ctx.HTML(200, tplSettingsPassword)
}

// SettingsPasswordPost response for change user's password
func SettingsPasswordPost(ctx *context.Context, form auth.ChangePasswordForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsPassword"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplSettingsPassword)
		return
	}

	if !ctx.User.ValidatePassword(form.OldPassword) {
		ctx.Flash.Error(ctx.Tr("settings.password_incorrect"))
	} else if form.Password != form.Retype {
		ctx.Flash.Error(ctx.Tr("form.password_not_match"))
	} else {
		ctx.User.Passwd = form.Password
		var err error
		if ctx.User.Salt, err = models.GetUserSalt(); err != nil {
			ctx.Handle(500, "UpdateUser", err)
			return
		}
		ctx.User.EncodePasswd()
		if err := models.UpdateUser(ctx.User); err != nil {
			ctx.Handle(500, "UpdateUser", err)
			return
		}
		log.Trace("User password updated: %s", ctx.User.Name)
		ctx.Flash.Success(ctx.Tr("settings.change_password_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/password")
}

// SettingsEmails render user's emails page
func SettingsEmails(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsEmails"] = true

	emails, err := models.GetEmailAddresses(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "GetEmailAddresses", err)
		return
	}
	ctx.Data["Emails"] = emails

	ctx.HTML(200, tplSettingsEmails)
}

// SettingsEmailPost response for change user's email
func SettingsEmailPost(ctx *context.Context, form auth.AddEmailForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsEmails"] = true

	// Make emailaddress primary.
	if ctx.Query("_method") == "PRIMARY" {
		if err := models.MakeEmailPrimary(&models.EmailAddress{ID: ctx.QueryInt64("id")}); err != nil {
			ctx.Handle(500, "MakeEmailPrimary", err)
			return
		}

		log.Trace("Email made primary: %s", ctx.User.Name)
		ctx.Redirect(setting.AppSubURL + "/user/settings/email")
		return
	}

	// Add Email address.
	emails, err := models.GetEmailAddresses(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "GetEmailAddresses", err)
		return
	}
	ctx.Data["Emails"] = emails

	if ctx.HasError() {
		ctx.HTML(200, tplSettingsEmails)
		return
	}

	email := &models.EmailAddress{
		UID:         ctx.User.ID,
		Email:       form.Email,
		IsActivated: !setting.Service.RegisterEmailConfirm,
	}
	if err := models.AddEmailAddress(email); err != nil {
		if models.IsErrEmailAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplSettingsEmails, &form)
			return
		}
		ctx.Handle(500, "AddEmailAddress", err)
		return
	}

	// Send confirmation email
	if setting.Service.RegisterEmailConfirm {
		models.SendActivateEmailMail(ctx.Context, ctx.User, email)

		if err := ctx.Cache.Put("MailResendLimit_"+ctx.User.LowerName, ctx.User.LowerName, 180); err != nil {
			log.Error(4, "Set cache(MailResendLimit) fail: %v", err)
		}
		ctx.Flash.Info(ctx.Tr("settings.add_email_confirmation_sent", email.Email, setting.Service.ActiveCodeLives/60))
	} else {
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))
	}

	log.Trace("Email address added: %s", email.Email)
	ctx.Redirect(setting.AppSubURL + "/user/settings/email")
}

// DeleteEmail response for delete user's email
func DeleteEmail(ctx *context.Context) {
	if err := models.DeleteEmailAddress(&models.EmailAddress{ID: ctx.QueryInt64("id"), UID: ctx.User.ID}); err != nil {
		ctx.Handle(500, "DeleteEmail", err)
		return
	}
	log.Trace("Email address deleted: %s", ctx.User.Name)

	ctx.Flash.Success(ctx.Tr("settings.email_deletion_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/email",
	})
}

// SettingsSSHKeys render user's SSH public keys page
func SettingsSSHKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSSHKeys"] = true

	keys, err := models.ListPublicKeys(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "ListPublicKeys", err)
		return
	}
	ctx.Data["Keys"] = keys

	ctx.HTML(200, tplSettingsSSHKeys)
}

// SettingsSSHKeysPost response for change user's SSH keys
func SettingsSSHKeysPost(ctx *context.Context, form auth.AddSSHKeyForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSSHKeys"] = true

	keys, err := models.ListPublicKeys(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "ListPublicKeys", err)
		return
	}
	ctx.Data["Keys"] = keys

	if ctx.HasError() {
		ctx.HTML(200, tplSettingsSSHKeys)
		return
	}

	content, err := models.CheckPublicKeyString(form.Content)
	if err != nil {
		if models.IsErrKeyUnableVerify(err) {
			ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
		} else {
			ctx.Flash.Error(ctx.Tr("form.invalid_ssh_key", err.Error()))
			ctx.Redirect(setting.AppSubURL + "/user/settings/ssh")
			return
		}
	}

	if _, err = models.AddPublicKey(ctx.User.ID, form.Title, content); err != nil {
		ctx.Data["HasError"] = true
		switch {
		case models.IsErrKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErr(ctx.Tr("settings.ssh_key_been_used"), tplSettingsSSHKeys, &form)
		case models.IsErrKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("settings.ssh_key_name_used"), tplSettingsSSHKeys, &form)
		default:
			ctx.Handle(500, "AddPublicKey", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.add_key_success", form.Title))
	ctx.Redirect(setting.AppSubURL + "/user/settings/ssh")
}

// DeleteSSHKey response for delete user's SSH key
func DeleteSSHKey(ctx *context.Context) {
	if err := models.DeletePublicKey(ctx.User, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeletePublicKey: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.ssh_key_deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/ssh",
	})
}

// SettingsApplications render user's access tokens page
func SettingsApplications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	tokens, err := models.ListAccessTokens(ctx.User.ID)
	if err != nil {
		ctx.Handle(500, "ListAccessTokens", err)
		return
	}
	ctx.Data["Tokens"] = tokens

	ctx.HTML(200, tplSettingsApplications)
}

// SettingsApplicationsPost response for add user's access token
func SettingsApplicationsPost(ctx *context.Context, form auth.NewAccessTokenForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		tokens, err := models.ListAccessTokens(ctx.User.ID)
		if err != nil {
			ctx.Handle(500, "ListAccessTokens", err)
			return
		}
		ctx.Data["Tokens"] = tokens
		ctx.HTML(200, tplSettingsApplications)
		return
	}

	t := &models.AccessToken{
		UID:  ctx.User.ID,
		Name: form.Name,
	}
	if err := models.NewAccessToken(t); err != nil {
		ctx.Handle(500, "NewAccessToken", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.generate_token_succees"))
	ctx.Flash.Info(t.Sha1)

	ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
}

// SettingsDeleteApplication response for delete user access token
func SettingsDeleteApplication(ctx *context.Context) {
	if err := models.DeleteAccessTokenByID(ctx.QueryInt64("id"), ctx.User.ID); err != nil {
		ctx.Flash.Error("DeleteAccessTokenByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.delete_token_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}

// SettingsDelete render user suicide page and response for delete user himself
func SettingsDelete(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsDelete"] = true

	if ctx.Req.Method == "POST" {
		if _, err := models.UserSignIn(ctx.User.Name, ctx.Query("password")); err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_password"), tplSettingsDelete, nil)
			} else {
				ctx.Handle(500, "UserSignIn", err)
			}
			return
		}

		if err := models.DeleteUser(ctx.User); err != nil {
			switch {
			case models.IsErrUserOwnRepos(err):
				ctx.Flash.Error(ctx.Tr("form.still_own_repo"))
				ctx.Redirect(setting.AppSubURL + "/user/settings/delete")
			case models.IsErrUserHasOrgs(err):
				ctx.Flash.Error(ctx.Tr("form.still_has_org"))
				ctx.Redirect(setting.AppSubURL + "/user/settings/delete")
			default:
				ctx.Handle(500, "DeleteUser", err)
			}
		} else {
			log.Trace("Account deleted: %s", ctx.User.Name)
			ctx.Redirect(setting.AppSubURL + "/")
		}
		return
	}

	ctx.HTML(200, tplSettingsDelete)
}
