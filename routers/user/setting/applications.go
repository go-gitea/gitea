// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSettingsApplications      base.TplName = "user/settings/applications"
	tplSettingsOAuthApplications base.TplName = "user/settings/applications_oauth2_edit"
)

// Applications render manage access token page
func Applications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	loadApplicationsData(ctx)

	ctx.HTML(200, tplSettingsApplications)
}

// ApplicationsPost response for add user's access token
func ApplicationsPost(ctx *context.Context, form auth.NewAccessTokenForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		loadApplicationsData(ctx)

		ctx.HTML(200, tplSettingsApplications)
		return
	}

	t := &models.AccessToken{
		UID:  ctx.User.ID,
		Name: form.Name,
	}
	if err := models.NewAccessToken(t); err != nil {
		ctx.ServerError("NewAccessToken", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.generate_token_success"))
	ctx.Flash.Info(t.Sha1)

	ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
}

// OAuthApplicationsPost response for add user's access token
func OAuthApplicationsPost(ctx *context.Context, form auth.NewOAuth2ApplicationForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		loadApplicationsData(ctx)

		ctx.HTML(200, tplSettingsApplications)
		return
	}
	app, err := models.CreateOAuth2Application(models.CreateOAuth2ApplicationOptions{
		Name:         form.Name,
		RedirectURIs: []string{form.RedirectURI},
		UserID:       ctx.User.ID,
	})
	if err != nil {
		ctx.ServerError("CreateOAuth2Application", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.create_oauth2_application_success"))
	ctx.Data["App"] = app
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret()
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}
	ctx.HTML(200, tplSettingsOAuthApplications)
}

// DeleteApplication response for delete user access token
func DeleteApplication(ctx *context.Context) {
	if err := models.DeleteAccessTokenByID(ctx.QueryInt64("id"), ctx.User.ID); err != nil {
		ctx.Flash.Error("DeleteAccessTokenByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.delete_token_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}

func loadApplicationsData(ctx *context.Context) {
	tokens, err := models.ListAccessTokens(ctx.User.ID)
	if err != nil {
		ctx.ServerError("ListAccessTokens", err)
		return
	}
	ctx.Data["Tokens"] = tokens
	ctx.Data["EnableOAuth2"] = setting.OAuth2.Enable
	if setting.OAuth2.Enable {
		ctx.Data["Applications"], err = models.GetOAuth2ApplicationsByUserID(ctx.User.ID)
		if err != nil {
			ctx.ServerError("GetOAuth2ApplicationsByUserID", err)
			return
		}
	}
}
