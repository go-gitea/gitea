// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/http"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/auth"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/services/audit"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

type OAuth2CommonHandlers struct {
	Owner              *user_model.User  // nil for instance-wide, otherwise the Org or User owning the applications
	BasePathList       string            // the base URL for the application list page, eg: "/user/setting/applications"
	BasePathEditPrefix string            // the base URL for the application edit page, will be appended with app id, eg: "/user/setting/applications/oauth2"
	TplAppEdit         templates.TplName // the template for the application edit page
}

func (oa *OAuth2CommonHandlers) ownerID() int64 {
	if oa.Owner != nil {
		return oa.Owner.ID
	}
	return 0
}

// recordAudit emits an OAuth2 application audit event scoped to the owner, which
// is nil for instance-wide (admin) applications, an organization, or a user.
func (oa *OAuth2CommonHandlers) recordAudit(ctx *context.Context, actions audit.ScopedActions, appName string) {
	audit.RecordScoped(ctx, oa.Owner, nil, actions, "oauth2_application", appName)
}

func (oa *OAuth2CommonHandlers) renderEditPage(ctx *context.Context, app *auth.OAuth2Application) {
	ctx.Data["App"] = app
	ctx.Data["FormActionPath"] = fmt.Sprintf("%s/%d", oa.BasePathEditPrefix, app.ID)

	if ctx.ContextUser != nil && ctx.ContextUser.IsOrganization() {
		if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
			ctx.ServerError("RenderUserOrgHeader", err)
			return
		}
	}

	ctx.HTML(http.StatusOK, oa.TplAppEdit)
}

// AddApp adds an oauth2 application
func (oa *OAuth2CommonHandlers) AddApp(ctx *context.Context) {
	form := web.GetForm[*forms.EditOAuth2ApplicationForm](ctx)
	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		// go to the application list page
		ctx.Redirect(oa.BasePathList)
		return
	}

	app, err := auth.CreateOAuth2Application(ctx, auth.CreateOAuth2ApplicationOptions{
		Name:                       form.Name,
		RedirectURIs:               util.SplitTrimSpace(form.RedirectURIs, "\n"),
		UserID:                     oa.ownerID(),
		ConfidentialClient:         form.ConfidentialClient,
		SkipSecondaryAuthorization: form.SkipSecondaryAuthorization,
	})
	if err != nil {
		ctx.ServerError("CreateOAuth2Application", err)
		return
	}

	oa.recordAudit(ctx, audit.ScopedActions{
		User:   audit_model.UserOAuth2ApplicationAdd,
		Org:    audit_model.OrganizationOAuth2ApplicationAdd,
		System: audit_model.SystemOAuth2ApplicationAdd,
	}, app.Name)

	// render the edit page with secret
	ctx.Flash.Success(ctx.Tr("settings.create_oauth2_application_success"), true)
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret(ctx)
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}

	oa.renderEditPage(ctx, app)
}

// EditShow displays the given application
func (oa *OAuth2CommonHandlers) EditShow(ctx *context.Context) {
	app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != oa.ownerID() {
		ctx.NotFound(nil)
		return
	}
	oa.renderEditPage(ctx, app)
}

// EditSave saves the oauth2 application
func (oa *OAuth2CommonHandlers) EditSave(ctx *context.Context) {
	form := web.GetForm[*forms.EditOAuth2ApplicationForm](ctx)

	if ctx.HasError() {
		app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.PathParamInt64("id"))
		if err != nil {
			if auth.IsErrOAuthApplicationNotFound(err) {
				ctx.NotFound(err)
				return
			}
			ctx.ServerError("GetOAuth2ApplicationByID", err)
			return
		}
		if app.UID != oa.ownerID() {
			ctx.NotFound(nil)
			return
		}
		oa.renderEditPage(ctx, app)
		return
	}

	updatedApp, err := auth.UpdateOAuth2Application(ctx, auth.UpdateOAuth2ApplicationOptions{
		ID:                         ctx.PathParamInt64("id"),
		Name:                       form.Name,
		RedirectURIs:               util.SplitTrimSpace(form.RedirectURIs, "\n"),
		UserID:                     oa.ownerID(),
		ConfidentialClient:         form.ConfidentialClient,
		SkipSecondaryAuthorization: form.SkipSecondaryAuthorization,
	})
	if err != nil {
		ctx.ServerError("UpdateOAuth2Application", err)
		return
	}

	oa.recordAudit(ctx, audit.ScopedActions{
		User:   audit_model.UserOAuth2ApplicationUpdate,
		Org:    audit_model.OrganizationOAuth2ApplicationUpdate,
		System: audit_model.SystemOAuth2ApplicationUpdate,
	}, updatedApp.Name)

	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"))
	ctx.Redirect(oa.BasePathList)
}

// RegenerateSecret regenerates the secret
func (oa *OAuth2CommonHandlers) RegenerateSecret(ctx *context.Context) {
	app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		if auth.IsErrOAuthApplicationNotFound(err) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByID", err)
		return
	}
	if app.UID != oa.ownerID() {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["ClientSecret"], err = app.GenerateClientSecret(ctx)
	if err != nil {
		ctx.ServerError("GenerateClientSecret", err)
		return
	}

	oa.recordAudit(ctx, audit.ScopedActions{
		User:   audit_model.UserOAuth2ApplicationSecret,
		Org:    audit_model.OrganizationOAuth2ApplicationSecret,
		System: audit_model.SystemOAuth2ApplicationSecret,
	}, app.Name)

	ctx.Flash.Success(ctx.Tr("settings.update_oauth2_application_success"), true)
	oa.renderEditPage(ctx, app)
}

// DeleteApp deletes the given oauth2 application
func (oa *OAuth2CommonHandlers) DeleteApp(ctx *context.Context) {
	app, err := auth.GetOAuth2ApplicationByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetOAuth2ApplicationByID", auth.IsErrOAuthApplicationNotFound, err)
		return
	}

	if err := auth.DeleteOAuth2Application(ctx, app.ID, oa.ownerID()); err != nil {
		ctx.ServerError("DeleteOAuth2Application", err)
		return
	}

	oa.recordAudit(ctx, audit.ScopedActions{
		User:   audit_model.UserOAuth2ApplicationRemove,
		Org:    audit_model.OrganizationOAuth2ApplicationRemove,
		System: audit_model.SystemOAuth2ApplicationRemove,
	}, app.Name)

	ctx.Flash.Success(ctx.Tr("settings.remove_oauth2_application_success"))
	ctx.JSONRedirect(oa.BasePathList)
}

// RevokeGrant revokes the grant
func (oa *OAuth2CommonHandlers) RevokeGrant(ctx *context.Context) {
	if oa.Owner == nil || oa.Owner.IsOrganization() {
		ctx.NotFound(nil)
		return
	}

	grant, err := auth.GetOAuth2GrantByID(ctx, ctx.PathParamInt64("grantId"))
	if err != nil {
		ctx.ServerError("GetOAuth2GrantByID", err)
		return
	}
	if grant == nil || grant.UserID != oa.ownerID() {
		ctx.NotFound(nil)
		return
	}

	app, err := auth.GetOAuth2ApplicationByID(ctx, grant.ApplicationID)
	if err != nil {
		ctx.NotFoundOrServerError("GetOAuth2ApplicationByID", auth.IsErrOAuthApplicationNotFound, err)
		return
	}

	if err := auth.RevokeOAuth2Grant(ctx, grant.ID, oa.ownerID()); err != nil {
		ctx.ServerError("RevokeOAuth2Grant", err)
		return
	}

	oa.recordAudit(ctx, audit.ScopedActions{
		User: audit_model.UserOAuth2ApplicationRevoke,
	}, app.Name)

	ctx.Flash.Success(ctx.Tr("settings.revoke_oauth2_grant_success"))
	ctx.JSONRedirect(oa.BasePathList)
}
