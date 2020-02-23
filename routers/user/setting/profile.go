// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
	"github.com/unknwon/i18n"
)

const (
	tplSettingsProfile      base.TplName = "user/settings/profile"
	tplSettingsOrganization base.TplName = "user/settings/organization"
	tplSettingsRepositories base.TplName = "user/settings/repos"
)

// Profile render user's profile page
func Profile(ctx *context.Context) {
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
				ctx.Flash.Error(ctx.Tr("form.username_been_taken"))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			case models.IsErrEmailAlreadyUsed(err):
				ctx.Flash.Error(ctx.Tr("form.email_been_used"))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			case models.IsErrNameReserved(err):
				ctx.Flash.Error(ctx.Tr("user.form.name_reserved", newName))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			case models.IsErrNamePatternNotAllowed(err):
				ctx.Flash.Error(ctx.Tr("user.form.name_pattern_not_allowed", newName))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			case models.IsErrNameCharsNotAllowed(err):
				ctx.Flash.Error(ctx.Tr("user.form.name_chars_not_allowed", newName))
				ctx.Redirect(setting.AppSubURL + "/user/settings")
			default:
				ctx.ServerError("ChangeUserName", err)
			}
			return
		}
		log.Trace("User name changed: %s -> %s", ctx.User.Name, newName)
	}

	// In case it's just a case change
	ctx.User.Name = newName
	ctx.User.LowerName = strings.ToLower(newName)
}

// ProfilePost response for change user's profile
func ProfilePost(ctx *context.Context, form auth.UpdateProfileForm) {
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
	ctx.User.Language = form.Language
	ctx.User.Description = form.Description
	if err := models.UpdateUserSetting(ctx.User); err != nil {
		if _, ok := err.(models.ErrEmailAlreadyUsed); ok {
			ctx.Flash.Error(ctx.Tr("form.email_been_used"))
			ctx.Redirect(setting.AppSubURL + "/user/settings")
			return
		}
		ctx.ServerError("UpdateUser", err)
		return
	}

	// Update the language to the one we just set
	ctx.SetCookie("lang", ctx.User.Language, nil, setting.AppSubURL, setting.SessionConfig.Domain, setting.SessionConfig.Secure, true)

	log.Trace("User settings updated: %s", ctx.User.Name)
	ctx.Flash.Success(i18n.Tr(ctx.User.Language, "settings.update_profile_success"))
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

	if form.Avatar != nil && form.Avatar.Filename != "" {
		fr, err := form.Avatar.Open()
		if err != nil {
			return fmt.Errorf("Avatar.Open: %v", err)
		}
		defer fr.Close()

		if form.Avatar.Size > setting.AvatarMaxFileSize {
			return errors.New(ctx.Tr("settings.uploaded_avatar_is_too_big"))
		}

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
	} else if ctxUser.UseCustomAvatar && !com.IsFile(ctxUser.CustomAvatarPath()) {
		// No avatar is uploaded but setting has been changed to enable,
		// generate a random one when needed.
		if err := ctxUser.GenerateRandomAvatar(); err != nil {
			log.Error("GenerateRandomAvatar[%d]: %v", ctxUser.ID, err)
		}
	}

	if err := models.UpdateUserCols(ctxUser, "avatar", "avatar_email", "use_custom_avatar"); err != nil {
		return fmt.Errorf("UpdateUser: %v", err)
	}

	return nil
}

// AvatarPost response for change user's avatar request
func AvatarPost(ctx *context.Context, form auth.AvatarForm) {
	if err := UpdateAvatarSetting(ctx, form, ctx.User); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.update_avatar_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// DeleteAvatar render delete avatar page
func DeleteAvatar(ctx *context.Context) {
	if err := ctx.User.DeleteAvatar(); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// Organization render all the organization of the user
func Organization(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsOrganization"] = true
	orgs, err := models.GetOrgsByUserID(ctx.User.ID, ctx.IsSigned)
	if err != nil {
		ctx.ServerError("GetOrgsByUserID", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	ctx.HTML(200, tplSettingsOrganization)
}

// Repos display a list of all repositories of the user
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsRepos"] = true
	ctxUser := ctx.User

	var err error
	if err = ctxUser.GetRepositories(1, setting.UI.User.RepoPagingNum); err != nil {
		ctx.ServerError("GetRepositories", err)
		return
	}
	repos := ctxUser.Repos

	for i := range repos {
		if repos[i].IsFork {
			err := repos[i].GetBaseRepo()
			if err != nil {
				ctx.ServerError("GetBaseRepo", err)
				return
			}
			err = repos[i].BaseRepo.GetOwner()
			if err != nil {
				ctx.ServerError("GetOwner", err)
				return
			}
		}
	}

	ctx.Data["Owner"] = ctxUser
	ctx.Data["Repos"] = repos

	ctx.HTML(200, tplSettingsRepositories)
}
