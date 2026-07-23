// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

const (
	tplAdminCodespaceManagers templates.TplName = "codespace/admin_managers"
	tplUserCodespaceSettings  templates.TplName = "codespace/user_settings"
	tplOrgCodespaceSettings   templates.TplName = "codespace/org_settings"
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

// AdminManagersPost handles site-wide Manager and global registration token actions.
func AdminManagersPost(ctx *context.Context) {
	handleManagerSettingsPost(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeSite,
		ActionBase: setting.AppSubURL + "/-/admin/codespaces/managers",
		Template:   tplAdminCodespaceManagers,
		PageFlag:   "PageIsAdminCodespaceManagers",
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
		OwnerID:    ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces",
		Template:   tplUserCodespaceSettings,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

// UserSettingsPost handles current user's Manager and registration token actions.
func UserSettingsPost(ctx *context.Context) {
	handleManagerSettingsPost(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeUser,
		OwnerID:    ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces",
		Template:   tplUserCodespaceSettings,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

// UserSettingsResetRegistrationToken resets the current user's Manager registration token.
func UserSettingsResetRegistrationToken(ctx *context.Context) {
	handleManagerSettingsResetRegistrationToken(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeUser,
		OwnerID:    ctx.Doer.ID,
		ActionBase: setting.AppSubURL + "/user/settings/codespaces",
		Template:   tplUserCodespaceSettings,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

// OrgList renders organization Codespace governance, Manager and registration token settings.
func OrgList(ctx *context.Context) {
	if ctx.Org == nil || ctx.Org.Organization == nil {
		ctx.NotFound(nil)
		return
	}
	renderManagerSettings(ctx, managerSettingsRenderOptions{
		Scope:             codespace_service.ManagerSettingsScopeOrganization,
		OwnerID:           ctx.Org.Organization.ID,
		ActionBase:        ctx.Org.OrgLink + "/settings/codespaces",
		Template:          tplOrgCodespaceSettings,
		PageFlag:          "PageIsCodespaceSettings",
		IncludeGovernance: true,
		GovernanceScope:   codespace_service.GovernanceScopeOrganization,
	})
}

// OrgSettingsPost handles organization Manager and registration token actions.
func OrgSettingsPost(ctx *context.Context) {
	if ctx.Org == nil || ctx.Org.Organization == nil {
		ctx.NotFound(nil)
		return
	}
	handleManagerSettingsPost(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeOrganization,
		OwnerID:    ctx.Org.Organization.ID,
		ActionBase: ctx.Org.OrgLink + "/settings/codespaces",
		Template:   tplOrgCodespaceSettings,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

// OrgSettingsResetRegistrationToken resets the organization's Manager registration token.
func OrgSettingsResetRegistrationToken(ctx *context.Context) {
	if ctx.Org == nil || ctx.Org.Organization == nil {
		ctx.NotFound(nil)
		return
	}
	handleManagerSettingsResetRegistrationToken(ctx, managerSettingsRenderOptions{
		Scope:      codespace_service.ManagerSettingsScopeOrganization,
		OwnerID:    ctx.Org.Organization.ID,
		ActionBase: ctx.Org.OrgLink + "/settings/codespaces",
		Template:   tplOrgCodespaceSettings,
		PageFlag:   "PageIsCodespaceSettings",
	})
}

type managerSettingsRenderOptions struct {
	Scope             string
	OwnerID           int64
	ActionBase        string
	Template          templates.TplName
	PageFlag          string
	IncludeGovernance bool
	GovernanceScope   string
}

func renderManagerSettings(ctx *context.Context, opts managerSettingsRenderOptions) {
	settingsView, err := codespace_service.ListManagerSettings(ctx, codespace_service.ManagerSettingsOptions{
		Scope:   opts.Scope,
		OwnerID: opts.OwnerID,
	})
	if err != nil {
		ctx.ServerError("ListManagerSettings", err)
		return
	}
	if opts.IncludeGovernance {
		result, err := codespace_service.ListGovernanceCodespaces(ctx, codespace_service.GovernanceListOptions{
			Scope:   opts.GovernanceScope,
			OwnerID: opts.OwnerID,
		})
		if err != nil {
			ctx.ServerError("ListGovernanceCodespaces", err)
			return
		}
		ctx.Data["Codespaces"] = result.Rows
		ctx.Data["IsSiteGovernance"] = false
	}
	ctx.Data["Title"] = "Codespaces"
	ctx.Data[opts.PageFlag] = true
	ctx.Data["ManagerSettings"] = settingsView
	ctx.Data["ManagerTotal"] = len(settingsView.Managers)
	ctx.Data["ActionBase"] = opts.ActionBase
	ctx.Data["IsSiteManagerSettings"] = opts.Scope == codespace_service.ManagerSettingsScopeSite
	ctx.HTML(http.StatusOK, opts.Template)
}

func handleManagerSettingsPost(ctx *context.Context, opts managerSettingsRenderOptions) {
	var err error
	switch ctx.FormString("action") {
	case "delete_manager":
		err = codespace_service.DeleteManager(ctx, codespace_service.DeleteManagerOptions{
			Scope:     opts.Scope,
			OwnerID:   opts.OwnerID,
			ManagerID: ctx.FormInt64("manager_id"),
			Confirm:   ctx.FormString("confirm") == "delete-manager",
		})
	default:
		ctx.PlainText(http.StatusBadRequest, "unknown_action")
		return
	}
	if err != nil {
		handleManagerSettingsActionError(ctx, err)
		return
	}
	ctx.Redirect(opts.ActionBase, http.StatusSeeOther)
}

func handleManagerSettingsResetRegistrationToken(ctx *context.Context, opts managerSettingsRenderOptions) {
	_, err := codespace_service.ResetRegistrationToken(ctx, codespace_service.ManagerSettingsOptions{
		Scope:   opts.Scope,
		OwnerID: opts.OwnerID,
	})
	if err != nil {
		handleManagerSettingsActionError(ctx, err)
		return
	}
	ctx.Flash.Success("Registration token has been reset.")
	ctx.JSONRedirect(opts.ActionBase)
}

func handleManagerSettingsActionError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrManagerSettingsNotFound):
		ctx.NotFound(nil)
	case errors.Is(err, codespace_service.ErrManagerSettingsConfirmRequired):
		ctx.PlainText(http.StatusBadRequest, "confirm_required")
	default:
		ctx.ServerError("CodespaceManagerSettingsAction", err)
	}
}
