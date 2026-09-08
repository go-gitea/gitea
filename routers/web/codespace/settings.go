// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"
	"strconv"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

const (
	tplAdminCodespaceManagers      templates.TplName = "codespace/admin_managers"
	tplAdminCodespaceManagerDetail templates.TplName = "codespace/admin_manager_detail"
	tplAdminDevContainerTemplates  templates.TplName = "codespace/admin_devcontainer_templates"
	tplUserCodespaceSettings       templates.TplName = "codespace/user_settings"
	tplUserCodespaceManagerDetail  templates.TplName = "codespace/user_manager_detail"
	tplUserDevContainerTemplates   templates.TplName = "codespace/user_devcontainer_templates"
)

// AdminManagers renders site-wide Manager settings.
func AdminManagers(ctx *context.Context) {
	renderManagerSettings(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeSite,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/managers",
		Template:   tplAdminCodespaceManagers,
		PageFlag:   "PageIsAdminCodespaceManagers",
	})
}

// AdminManager renders one site-visible Manager and its bound Codespaces.
func AdminManager(ctx *context.Context) {
	renderManagerDetail(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeSite,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/managers",
		Template:   tplAdminCodespaceManagerDetail,
		PageFlag:   "PageIsAdminCodespaceManagers",
	})
}

// AdminManagerDelete removes one site-visible Manager from its management page.
func AdminManagerDelete(ctx *context.Context) {
	handleManagerDelete(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeSite,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/managers",
	})
}

func AdminManagerUpdate(ctx *context.Context) {
	handleManagerUpdate(ctx, codespace_service.ManagerSettingsOptions{Scope: codespace_service.ManagerSettingsScopeSite})
}

func AdminManagerSecret(ctx *context.Context) {
	handleManagerSecret(ctx, codespace_service.ManagerSettingsOptions{Scope: codespace_service.ManagerSettingsScopeSite})
}

// AdminManagersCreateManager creates a site-wide Manager identity.
func AdminManagersCreateManager(ctx *context.Context) {
	handleManagerSettingsCreateManager(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeSite,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/managers",
	})
}

func AdminDevContainerTemplates(ctx *context.Context) {
	renderDevContainerTemplateSettings(ctx, devContainerTemplateRenderOptions{
		UserID:     0,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/dev-container-templates",
		Template:   tplAdminDevContainerTemplates,
		PageFlag:   "PageIsAdminCodespaceDevContainerTemplates",
	})
}

func AdminDevContainerTemplatePost(ctx *context.Context) {
	handleDevContainerTemplateUpsert(ctx, devContainerTemplateRenderOptions{
		UserID:     0,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/dev-container-templates",
	}, 0)
}

func AdminDevContainerTemplateUpdate(ctx *context.Context) {
	handleDevContainerTemplateUpsert(ctx, devContainerTemplateRenderOptions{
		UserID:     0,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/dev-container-templates",
	}, ctx.PathParamInt64("template_id"))
}

func AdminDevContainerTemplateDelete(ctx *context.Context) {
	handleDevContainerTemplateDelete(ctx, devContainerTemplateRenderOptions{
		UserID:     0,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/dev-container-templates",
	})
}

// UserSettings renders current user's Manager settings.
func UserSettings(ctx *context.Context) {
	renderManagerSettings(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeUser,
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/managers",
		Template:   tplUserCodespaceSettings,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

// UserManager renders one Manager owned by the current user and its bound Codespaces.
func UserManager(ctx *context.Context) {
	renderManagerDetail(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeUser,
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/managers",
		Template:   tplUserCodespaceManagerDetail,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

// UserManagerDelete removes one Manager owned by the current user.
func UserManagerDelete(ctx *context.Context) {
	handleManagerDelete(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeUser,
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/managers",
	})
}

func UserManagerUpdate(ctx *context.Context) {
	handleManagerUpdate(ctx, codespace_service.ManagerSettingsOptions{Scope: codespace_service.ManagerSettingsScopeUser, UserID: ctx.Doer.ID})
}

func UserManagerSecret(ctx *context.Context) {
	handleManagerSecret(ctx, codespace_service.ManagerSettingsOptions{Scope: codespace_service.ManagerSettingsScopeUser, UserID: ctx.Doer.ID})
}

// UserSettingsCreateManager creates a Manager identity owned by the current user.
func UserSettingsCreateManager(ctx *context.Context) {
	handleManagerSettingsCreateManager(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeUser,
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/managers",
	})
}

func UserDevContainerTemplates(ctx *context.Context) {
	renderDevContainerTemplateSettings(ctx, devContainerTemplateRenderOptions{
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/dev-container-templates",
		Template:   tplUserDevContainerTemplates,
		PageFlag:   "PageIsCodespaceDevContainerTemplates",
	})
}

func UserDevContainerTemplatePost(ctx *context.Context) {
	handleDevContainerTemplateUpsert(ctx, devContainerTemplateRenderOptions{
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/dev-container-templates",
	}, 0)
}

func UserDevContainerTemplateUpdate(ctx *context.Context) {
	handleDevContainerTemplateUpsert(ctx, devContainerTemplateRenderOptions{
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/dev-container-templates",
	}, ctx.PathParamInt64("template_id"))
}

func UserDevContainerTemplateDelete(ctx *context.Context) {
	handleDevContainerTemplateDelete(ctx, devContainerTemplateRenderOptions{
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/dev-container-templates",
	})
}

type managerSettingsRenderOptions struct {
	Scope      string
	UserID     int64
	ActionBase string
	Template   templates.TplName
	PageFlag   string
}

type devContainerTemplateRenderOptions struct {
	UserID     int64
	ActionBase string
	Template   templates.TplName
	PageFlag   string
}

func renderDevContainerTemplateSettings(ctx *context.Context, opts devContainerTemplateRenderOptions) {
	templates, err := codespace_service.ListDevContainerTemplates(ctx, opts.UserID)
	if err != nil {
		ctx.ServerError("ListDevContainerTemplates", err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("codespace.dev_container_templates")
	ctx.Data[opts.PageFlag] = true
	ctx.Data["DevContainerTemplates"] = templates
	ctx.Data["IsSiteTemplateSettings"] = opts.UserID == 0
	ctx.Data["ActionBase"] = opts.ActionBase
	ctx.HTML(http.StatusOK, opts.Template)
}

func handleDevContainerTemplateUpsert(ctx *context.Context, opts devContainerTemplateRenderOptions, templateID int64) {
	err := codespace_service.UpsertDevContainerTemplate(ctx, codespace_service.DevContainerTemplateUpsertOptions{
		UserID:  opts.UserID,
		ID:      templateID,
		Name:    ctx.FormString("name"),
		Content: ctx.FormString("content"),
	})
	if err != nil {
		handleDevContainerTemplateActionError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.saved_successfully"))
	ctx.JSONRedirect(opts.ActionBase)
}

func handleDevContainerTemplateDelete(ctx *context.Context, opts devContainerTemplateRenderOptions) {
	err := codespace_service.DeleteDevContainerTemplate(ctx, codespace_service.DevContainerTemplateDeleteOptions{
		UserID: opts.UserID,
		ID:     ctx.PathParamInt64("template_id"),
	})
	if err != nil {
		handleDevContainerTemplateActionError(ctx, err)
		return
	}
	ctx.JSONRedirect(opts.ActionBase)
}

func handleDevContainerTemplateActionError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrDevContainerTemplateNotFound):
		ctx.JSONErrorNotFound()
	case errors.Is(err, codespace_service.ErrDevContainerTemplateNameInvalid):
		ctx.JSONErrorWithField(ctx.Tr("codespace.dev_container_template_name_invalid"), "name")
	case errors.Is(err, codespace_service.ErrCreateConfigurationInvalid):
		ctx.JSONErrorWithField(err.Error(), "content")
	default:
		ctx.JSONErrorAuto(err)
	}
}

func renderManagerSettings(ctx *context.Context, opts managerSettingsRenderOptions) {
	if !populateManagerSettingsData(ctx, opts) {
		return
	}
	ctx.HTML(http.StatusOK, opts.Template)
}

func populateManagerSettingsData(ctx *context.Context, opts managerSettingsRenderOptions) bool {
	settingsView, err := codespace_service.ListManagerSettings(ctx, codespace_service.ManagerSettingsOptions{
		Scope:  opts.Scope,
		UserID: opts.UserID,
	})
	if err != nil {
		ctx.ServerError("ListManagerSettings", err)
		return false
	}
	ctx.Data["Title"] = "Codespaces"
	ctx.Data[opts.PageFlag] = true
	ctx.Data["ManagerSettings"] = settingsView
	ctx.Data["ManagerTotal"] = len(settingsView.Managers)
	ctx.Data["ActionBase"] = opts.ActionBase
	ctx.Data["IsSiteManagerSettings"] = opts.Scope == codespace_service.ManagerSettingsScopeSite
	if opts.Scope == codespace_service.ManagerSettingsScopeSite {
		page := max(ctx.FormInt("page"), 1)
		unassigned, err := codespace_service.ListGovernanceCodespaces(ctx, codespace_service.GovernanceListOptions{
			Unassigned: true,
			Page:       page,
			PageSize:   setting.UI.Admin.UserPagingNum,
		})
		if err != nil {
			ctx.ServerError("ListUnassignedCodespaces", err)
			return false
		}
		ctx.Data["Codespaces"] = unassigned.Rows
		ctx.Data["CodespaceTotal"] = unassigned.Total
		ctx.Data["CodespaceEmptyMessage"] = ctx.Tr("codespace.no_unassigned_codespaces")
		ctx.Data["CodespaceActionBase"] = opts.ActionBase + "/unassigned"
		ctx.Data["Page"] = context.NewPagerBuilder(ctx).TotalCount(unassigned.Total).PerPageLimit(setting.UI.Admin.UserPagingNum).CurPage(page).Build()
	}
	return true
}

func renderManagerDetail(ctx *context.Context, opts managerSettingsRenderOptions) {
	page := max(ctx.FormInt("page"), 1)
	detail, err := codespace_service.GetManagerDetail(ctx, codespace_service.ManagerDetailOptions{
		ManagerSettingsOptions: codespace_service.ManagerSettingsOptions{Scope: opts.Scope, UserID: opts.UserID},
		ManagerID:              ctx.PathParamInt64("manager_id"),
		Page:                   page,
		PageSize:               setting.UI.Admin.UserPagingNum,
	})
	if err != nil {
		if errors.Is(err, codespace_service.ErrManagerSettingsNotFound) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("GetManagerDetail", err)
		}
		return
	}
	ctx.Data["Title"] = detail.Manager.Name
	ctx.Data[opts.PageFlag] = true
	ctx.Data["Manager"] = detail.Manager
	ctx.Data["Codespaces"] = detail.Codespaces
	ctx.Data["CodespaceTotal"] = detail.Total
	ctx.Data["CodespaceEmptyMessage"] = ctx.Tr("codespace.no_bound_codespaces")
	ctx.Data["ActionBase"] = opts.ActionBase
	ctx.Data["IsSiteManagerSettings"] = opts.Scope == codespace_service.ManagerSettingsScopeSite
	ctx.Data["CodespaceActionBase"] = opts.ActionBase + "/" + strconv.FormatInt(detail.Manager.ID, 10) + "/codespaces"
	ctx.Data["Page"] = context.NewPagerBuilder(ctx).TotalCount(detail.Total).PerPageLimit(setting.UI.Admin.UserPagingNum).CurPage(page).Build()
	ctx.HTML(http.StatusOK, opts.Template)
}

func handleManagerDelete(ctx *context.Context, opts managerSettingsRenderOptions) {
	err := codespace_service.DeleteManager(ctx, codespace_service.DeleteManagerOptions{
		Scope:     opts.Scope,
		UserID:    opts.UserID,
		ManagerID: ctx.PathParamInt64("manager_id"),
		Confirm:   ctx.FormString("confirm") == "delete-manager",
	})
	if err != nil {
		handleManagerSettingsActionError(ctx, opts.ActionBase+"/"+ctx.PathParam("manager_id"), err)
		return
	}
	ctx.JSONRedirect(opts.ActionBase)
}

func handleManagerSettingsCreateManager(ctx *context.Context, opts managerSettingsRenderOptions) {
	managerID, err := codespace_service.CreateManager(ctx, codespace_service.CreateManagerOptions{
		ManagerSettingsOptions: codespace_service.ManagerSettingsOptions{
			Scope:  opts.Scope,
			UserID: opts.UserID,
		},
		Name: ctx.FormString("name"),
	})
	if err != nil {
		handleManagerEditError(ctx, err)
		return
	}
	ctx.JSONRedirect(opts.ActionBase + "/" + strconv.FormatInt(managerID, 10))
}

func handleManagerUpdate(ctx *context.Context, opts codespace_service.ManagerSettingsOptions) {
	if err := codespace_service.UpdateManagerName(ctx, opts, ctx.PathParamInt64("manager_id"), ctx.FormString("name")); err != nil {
		handleManagerEditError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.saved_successfully"))
	ctx.JSONRedirect(ctx.Req.URL.Path)
}

func handleManagerSecret(ctx *context.Context, opts codespace_service.ManagerSettingsOptions) {
	ctx.RespHeader().Set("Cache-Control", "no-store")
	secret, err := codespace_service.ResetManagerSecret(ctx, opts, ctx.PathParamInt64("manager_id"), ctx.FormString("confirm") == "reset-secret")
	if err != nil {
		handleManagerEditError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]string{"secret": secret})
}

func handleManagerEditError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrManagerSettingsNotFound):
		ctx.JSONErrorNotFound()
	case errors.Is(err, codespace_service.ErrManagerSettingsNameInvalid):
		ctx.JSONErrorWithField(ctx.Tr("codespace.manager_name_invalid"), "name")
	case errors.Is(err, codespace_service.ErrManagerSettingsConfirmRequired):
		ctx.JSONError(ctx.Tr("codespace.error.confirm_required"))
	case errors.Is(err, codespace_service.ErrManagerSettingsChanged):
		ctx.JSONError(ctx.Tr("codespace.manager_settings_changed"))
	default:
		ctx.JSONErrorAuto(err)
	}
}

func handleManagerSettingsActionError(ctx *context.Context, redirectTo string, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrManagerSettingsNotFound):
		ctx.NotFound(nil)
	case errors.Is(err, codespace_service.ErrManagerSettingsConfirmRequired):
		ctx.Flash.Error(ctx.Tr("codespace.error.confirm_required"))
		ctx.Redirect(redirectTo, http.StatusSeeOther)
	case errors.Is(err, codespace_service.ErrManagerSettingsOwnershipConflict):
		ctx.Flash.Error(ctx.Tr("codespace.manager_ownership_conflict"))
		ctx.Redirect(redirectTo, http.StatusSeeOther)
	default:
		ctx.ServerError("CodespaceManagerSettingsAction", err)
	}
}
