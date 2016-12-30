// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/user"
)

func parseLoginSource(ctx *context.APIContext, u *models.User, sourceID int64, loginName string) {
	if sourceID == 0 {
		return
	}

	source, err := models.GetLoginSourceByID(sourceID)
	if err != nil {
		if models.IsErrLoginSourceNotExist(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "GetLoginSourceByID", err)
		}
		return
	}

	u.LoginType = source.Type
	u.LoginSource = source.ID
	u.LoginName = loginName
}

// CreateUser api for creating a user
// see https://github.com/gogits/go-gogs-client/wiki/Administration-Users#create-a-new-user
func CreateUser(ctx *context.APIContext, form api.CreateUserOption) {
	u := &models.User{
		Name:      form.Username,
		FullName:  form.FullName,
		Email:     form.Email,
		Passwd:    form.Password,
		IsActive:  true,
		LoginType: models.LoginPlain,
	}

	parseLoginSource(ctx, u, form.SourceID, form.LoginName)
	if ctx.Written() {
		return
	}

	if err := models.CreateUser(u); err != nil {
		if models.IsErrUserAlreadyExist(err) ||
			models.IsErrEmailAlreadyUsed(err) ||
			models.IsErrNameReserved(err) ||
			models.IsErrNamePatternNotAllowed(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "CreateUser", err)
		}
		return
	}
	log.Trace("Account created by admin (%s): %s", ctx.User.Name, u.Name)

	// Send email notification.
	if form.SendNotify && setting.MailService != nil {
		models.SendRegisterNotifyMail(ctx.Context.Context, u)
	}

	ctx.JSON(201, u.APIFormat())
}

// EditUser api for modifying a user's information
// see https://github.com/gogits/go-gogs-client/wiki/Administration-Users#edit-an-existing-user
func EditUser(ctx *context.APIContext, form api.EditUserOption) {
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	parseLoginSource(ctx, u, form.SourceID, form.LoginName)
	if ctx.Written() {
		return
	}

	if len(form.Password) > 0 {
		u.Passwd = form.Password
		var err error
		if u.Salt, err = models.GetUserSalt(); err != nil {
			ctx.Error(500, "UpdateUser", err)
			return
		}
		u.EncodePasswd()
	}

	u.LoginName = form.LoginName
	u.FullName = form.FullName
	u.Email = form.Email
	u.Website = form.Website
	u.Location = form.Location
	if form.Active != nil {
		u.IsActive = *form.Active
	}
	if form.Admin != nil {
		u.IsAdmin = *form.Admin
	}
	if form.AllowGitHook != nil {
		u.AllowGitHook = *form.AllowGitHook
	}
	if form.AllowImportLocal != nil {
		u.AllowImportLocal = *form.AllowImportLocal
	}
	if form.MaxRepoCreation != nil {
		u.MaxRepoCreation = *form.MaxRepoCreation
	}

	if err := models.UpdateUser(u); err != nil {
		if models.IsErrEmailAlreadyUsed(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "UpdateUser", err)
		}
		return
	}
	log.Trace("Account profile updated by admin (%s): %s", ctx.User.Name, u.Name)

	ctx.JSON(200, u.APIFormat())
}

// DeleteUser api for deleting a user
// https://github.com/gogits/go-gogs-client/wiki/Administration-Users#delete-a-user
func DeleteUser(ctx *context.APIContext) {
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	if err := models.DeleteUser(u); err != nil {
		if models.IsErrUserOwnRepos(err) ||
			models.IsErrUserHasOrgs(err) {
			ctx.Error(422, "", err)
		} else {
			ctx.Error(500, "DeleteUser", err)
		}
		return
	}
	log.Trace("Account deleted by admin(%s): %s", ctx.User.Name, u.Name)

	ctx.Status(204)
}

// CreatePublicKey api for creating a public key to a user
// see https://github.com/gogits/go-gogs-client/wiki/Administration-Users#create-a-public-key-for-user
func CreatePublicKey(ctx *context.APIContext, form api.CreateKeyOption) {
	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	user.CreateUserPublicKey(ctx, form, u.ID)
}
