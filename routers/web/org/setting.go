// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	user_setting "code.gitea.io/gitea/routers/web/user/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	org_service "code.gitea.io/gitea/services/org"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	// tplSettingsOptions template path for render settings
	tplSettingsOptions templates.TplName = "org/settings/options"
	// tplSettingsDelete template path for render delete repository
	tplSettingsDelete templates.TplName = "org/settings/delete"
	// tplSettingsHooks template path for render hook settings
	tplSettingsHooks templates.TplName = "org/settings/hooks"
	// tplSettingsLabels template path for render labels settings
	tplSettingsLabels templates.TplName = "org/settings/labels"
)

// Settings render the main settings page
func Settings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsOptions"] = true
	ctx.Data["CurrentVisibility"] = ctx.Org.Organization.Visibility
	ctx.Data["RepoAdminChangeTeamAccess"] = ctx.Org.Organization.RepoAdminChangeTeamAccess
	ctx.Data["ContextUser"] = ctx.ContextUser

	err := shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsOptions)
}

// SettingsPost response for settings change submitted
func SettingsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UpdateOrgSettingForm)
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsOptions"] = true
	ctx.Data["CurrentVisibility"] = ctx.Org.Organization.Visibility

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSettingsOptions)
		return
	}

	org := ctx.Org.Organization

	if org.Name != form.Name {
		if err := user_service.RenameUser(ctx, org.AsUser(), form.Name); err != nil {
			if user_model.IsErrUserAlreadyExist(err) {
				ctx.Data["Err_Name"] = true
				ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tplSettingsOptions, &form)
			} else if db.IsErrNameReserved(err) {
				ctx.Data["Err_Name"] = true
				ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(db.ErrNameReserved).Name), tplSettingsOptions, &form)
			} else if db.IsErrNamePatternNotAllowed(err) {
				ctx.Data["Err_Name"] = true
				ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tplSettingsOptions, &form)
			} else {
				ctx.ServerError("RenameUser", err)
			}
			return
		}

		ctx.Org.OrgLink = setting.AppSubURL + "/org/" + url.PathEscape(org.Name)
	}

	if form.Email != "" {
		if err := user_service.ReplacePrimaryEmailAddress(ctx, org.AsUser(), form.Email); err != nil {
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tplSettingsOptions, &form)
			return
		}
	}

	opts := &user_service.UpdateOptions{
		FullName:                  optional.Some(form.FullName),
		Description:               optional.Some(form.Description),
		Website:                   optional.Some(form.Website),
		Location:                  optional.Some(form.Location),
		Visibility:                optional.Some(form.Visibility),
		RepoAdminChangeTeamAccess: optional.Some(form.RepoAdminChangeTeamAccess),
	}
	if ctx.Doer.IsAdmin {
		opts.MaxRepoCreation = optional.Some(form.MaxRepoCreation)
	}

	visibilityChanged := org.Visibility != form.Visibility

	if err := user_service.UpdateUser(ctx, org.AsUser(), opts); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	// update forks visibility
	if visibilityChanged {
		repos, _, err := repo_model.GetUserRepositories(ctx, &repo_model.SearchRepoOptions{
			Actor: org.AsUser(), Private: true, ListOptions: db.ListOptions{Page: 1, PageSize: org.NumRepos},
		})
		if err != nil {
			ctx.ServerError("GetRepositories", err)
			return
		}
		for _, repo := range repos {
			repo.OwnerName = org.Name
			if err := repo_service.UpdateRepository(ctx, repo, true); err != nil {
				ctx.ServerError("UpdateRepository", err)
				return
			}
		}
	}

	log.Trace("Organization setting updated: %s", org.Name)
	ctx.Flash.Success(ctx.Tr("org.settings.update_setting_success"))
	ctx.Redirect(ctx.Org.OrgLink + "/settings")
}

// SettingsAvatar response for change avatar on settings page
func SettingsAvatar(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AvatarForm)
	form.Source = forms.AvatarLocal
	if err := user_setting.UpdateAvatarSetting(ctx, form, ctx.Org.Organization.AsUser()); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("org.settings.update_avatar_success"))
	}

	ctx.Redirect(ctx.Org.OrgLink + "/settings")
}

// SettingsDeleteAvatar response for delete avatar on settings page
func SettingsDeleteAvatar(ctx *context.Context) {
	if err := user_service.DeleteAvatar(ctx, ctx.Org.Organization.AsUser()); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.JSONRedirect(ctx.Org.OrgLink + "/settings")
}

// SettingsDelete response for deleting an organization
func SettingsDelete(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsDelete"] = true

	if ctx.Req.Method == "POST" {
		if ctx.Org.Organization.Name != ctx.FormString("org_name") {
			ctx.Data["Err_OrgName"] = true
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_org_name"), tplSettingsDelete, nil)
			return
		}

		if err := org_service.DeleteOrganization(ctx, ctx.Org.Organization, false); err != nil {
			if repo_model.IsErrUserOwnRepos(err) {
				ctx.Flash.Error(ctx.Tr("form.org_still_own_repo"))
				ctx.Redirect(ctx.Org.OrgLink + "/settings/delete")
			} else if packages_model.IsErrUserOwnPackages(err) {
				ctx.Flash.Error(ctx.Tr("form.org_still_own_packages"))
				ctx.Redirect(ctx.Org.OrgLink + "/settings/delete")
			} else {
				ctx.ServerError("DeleteOrganization", err)
			}
		} else {
			log.Trace("Organization deleted: %s", ctx.Org.Organization.Name)
			ctx.Redirect(setting.AppSubURL + "/")
		}
		return
	}

	err := shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsDelete)
}

// Webhooks render webhook list page
func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("org.settings")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Org.OrgLink + "/settings/hooks"
	ctx.Data["BaseLinkNew"] = ctx.Org.OrgLink + "/settings/hooks"
	ctx.Data["Description"] = ctx.Tr("org.settings.hooks_desc")

	ws, err := db.Find[webhook.Webhook](ctx, webhook.ListWebhookOptions{OwnerID: ctx.Org.Organization.ID})
	if err != nil {
		ctx.ServerError("ListWebhooksByOpts", err)
		return
	}

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.Data["Webhooks"] = ws
	ctx.HTML(http.StatusOK, tplSettingsHooks)
}

// DeleteWebhook response for delete webhook
func DeleteWebhook(ctx *context.Context) {
	if err := webhook.DeleteWebhookByOwnerID(ctx, ctx.Org.Organization.ID, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteWebhookByOwnerID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSONRedirect(ctx.Org.OrgLink + "/settings/hooks")
}

// Labels render organization labels page
func Labels(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.labels")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsLabels"] = true
	ctx.Data["LabelTemplateFiles"] = repo_module.LabelTemplateFiles

	err := shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	ctx.HTML(http.StatusOK, tplSettingsLabels)
}
