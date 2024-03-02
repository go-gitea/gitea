// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

const (
	tplSettingsOAuthApplicationEdit base.TplName = "user/settings/applications_oauth2_edit"
)

func newOAuth2CommonHandlers(u *user_model.User) *OAuth2CommonHandlers {
	return &OAuth2CommonHandlers{
		Owner:              u,
		BasePathList:       setting.AppSubURL + "/user/settings/applications",
		BasePathEditPrefix: setting.AppSubURL + "/user/settings/applications/oauth2",
		TplAppEdit:         tplSettingsOAuthApplicationEdit,
	}
}

// OAuthApplicationsPost response for adding a oauth2 application
func OAuthApplicationsPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	oa := newOAuth2CommonHandlers(ctx.Doer)
	oa.AddApp(ctx)
}

// OAuthApplicationsEdit response for editing oauth2 application
func OAuthApplicationsEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	oa := newOAuth2CommonHandlers(ctx.Doer)
	oa.EditSave(ctx)
}

// OAuthApplicationsRegenerateSecret handles the post request for regenerating the secret
func OAuthApplicationsRegenerateSecret(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	oa := newOAuth2CommonHandlers(ctx.Doer)
	oa.RegenerateSecret(ctx)
}

// OAuth2ApplicationShow displays the given application
func OAuth2ApplicationShow(ctx *context.Context) {
	oa := newOAuth2CommonHandlers(ctx.Doer)
	oa.EditShow(ctx)
}

// DeleteOAuth2Application deletes the given oauth2 application
func DeleteOAuth2Application(ctx *context.Context) {
	oa := newOAuth2CommonHandlers(ctx.Doer)
	oa.DeleteApp(ctx)
}

// RevokeOAuth2Grant revokes the grant with the given id
func RevokeOAuth2Grant(ctx *context.Context) {
	oa := newOAuth2CommonHandlers(ctx.Doer)
	oa.RevokeGrant(ctx)
}
