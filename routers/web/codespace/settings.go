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
	tplUserCodespaceSettings       templates.TplName = "codespace/user_settings"
	tplUserCodespaceManagerDetail  templates.TplName = "codespace/user_manager_detail"
)

// AdminManagers renders site-wide Manager and global registration token settings.
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

// AdminManagersResetRegistrationToken resets the global Manager registration token.
func AdminManagersResetRegistrationToken(ctx *context.Context) {
	handleManagerSettingsResetRegistrationToken(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeSite,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/managers",
		Template:   tplAdminCodespaceManagers,
		PageFlag:   "PageIsAdminCodespaceManagers",
	})
}

// UserSettings renders current user's Manager and registration token settings.
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

// UserSettingsResetRegistrationToken resets the current user's Manager registration token.
func UserSettingsResetRegistrationToken(ctx *context.Context) {
	handleManagerSettingsResetRegistrationToken(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeUser,
		UserID:     ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces/managers",
		Template:   tplUserCodespaceSettings,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

type managerSettingsRenderOptions struct {
	Scope      string
	UserID     int64
	ActionBase string
	Template   templates.TplName
	PageFlag   string
}

func renderManagerSettings(ctx *context.Context, opts managerSettingsRenderOptions) {
	settingsView, err := codespace_service.ListManagerSettings(ctx, codespace_service.ManagerSettingsOptions{
		Scope:  opts.Scope,
		UserID: opts.UserID,
	})
	if err != nil {
		ctx.ServerError("ListManagerSettings", err)
		return
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
			return
		}
		ctx.Data["Codespaces"] = unassigned.Rows
		ctx.Data["CodespaceTotal"] = unassigned.Total
		ctx.Data["CodespaceEmptyMessage"] = ctx.Tr("codespace.no_unassigned_codespaces")
		ctx.Data["CodespaceActionBase"] = opts.ActionBase + "/unassigned"
		ctx.Data["Page"] = context.NewPagination(unassigned.Total, setting.UI.Admin.UserPagingNum, page, 5)
	}
	ctx.HTML(http.StatusOK, opts.Template)
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
	ctx.Data["Page"] = context.NewPagination(detail.Total, setting.UI.Admin.UserPagingNum, page, 5)
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

func handleManagerSettingsResetRegistrationToken(ctx *context.Context, opts managerSettingsRenderOptions) {
	_, err := codespace_service.ResetRegistrationToken(ctx, codespace_service.ManagerSettingsOptions{
		Scope:  opts.Scope,
		UserID: opts.UserID,
	})
	if err != nil {
		handleManagerSettingsActionError(ctx, opts.ActionBase, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("codespace.registration_token_reset"))
	ctx.JSONRedirect(opts.ActionBase)
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
