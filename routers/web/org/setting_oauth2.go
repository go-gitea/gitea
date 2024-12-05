// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	organization_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	user_setting "code.gitea.io/gitea/routers/web/user/setting"
	"code.gitea.io/gitea/services/context"
)

const (
	tplSettingsApplications         base.TplName = "org/settings/applications"
	tplSettingsOAuthApplicationEdit base.TplName = "org/settings/applications_oauth2_edit"
)

func newOAuth2CommonHandlers(doer *user_model.User, org *organization_model.Organization) *user_setting.OAuth2CommonHandlers {
	return &user_setting.OAuth2CommonHandlers{
		Doer:               doer,
		Owner:              organization_model.UserFromOrg(org),
		BasePathList:       fmt.Sprintf("%s/org/%s/settings/applications", setting.AppSubURL, org.Name),
		BasePathEditPrefix: fmt.Sprintf("%s/org/%s/settings/applications/oauth2", setting.AppSubURL, org.Name),
		TplAppEdit:         tplSettingsOAuthApplicationEdit,
	}
}

// Applications render org applications page (for org, at the moment, there are only OAuth2 applications)
func Applications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsApplications"] = true

	apps, err := db.Find[auth.OAuth2Application](ctx, auth.FindOAuth2ApplicationsOptions{
		OwnerID: ctx.Org.Organization.ID,
	})
	if err != nil {
		ctx.ServerError("GetOAuth2ApplicationsByUserID", err)
		return
	}
	ctx.Data["Applications"] = apps

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsApplications)
}

// OAuthApplicationsPost response for adding an oauth2 application
func OAuthApplicationsPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsApplications"] = true

	oa := newOAuth2CommonHandlers(ctx.Doer, ctx.Org.Organization)
	oa.AddApp(ctx)
}

// OAuth2ApplicationShow displays the given application
func OAuth2ApplicationShow(ctx *context.Context) {
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsApplications"] = true

	oa := newOAuth2CommonHandlers(ctx.Doer, ctx.Org.Organization)
	oa.EditShow(ctx)
}

// OAuth2ApplicationEdit response for editing oauth2 application
func OAuth2ApplicationEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.applications")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsApplications"] = true

	oa := newOAuth2CommonHandlers(ctx.Doer, ctx.Org.Organization)
	oa.EditSave(ctx)
}

// OAuthApplicationsRegenerateSecret handles the post request for regenerating the secret
func OAuthApplicationsRegenerateSecret(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsApplications"] = true

	oa := newOAuth2CommonHandlers(ctx.Doer, ctx.Org.Organization)
	oa.RegenerateSecret(ctx)
}

// DeleteOAuth2Application deletes the given oauth2 application
func DeleteOAuth2Application(ctx *context.Context) {
	oa := newOAuth2CommonHandlers(ctx.Doer, ctx.Org.Organization)
	oa.DeleteApp(ctx)
}

// TODO: revokes the grant with the given id
