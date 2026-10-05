// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"regexp"
	"strings"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	webhook_service "gitea.dev/services/webhook"
)

const (
	tplAdminHookTypes    templates.TplName = "admin/hook_types"
	tplAdminHookTypeEdit templates.TplName = "admin/hook_type_edit"
)

var hookTypeNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)

// HookTypes lists webhook type definitions.
func HookTypes(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.hook_types")
	ctx.Data["PageIsAdminHookTypes"] = true
	types, err := webhook_model.ListAllHookTypes(ctx)
	if err != nil {
		ctx.ServerError("ListAllHookTypes", err)
		return
	}
	ctx.Data["HookTypes"] = types
	ctx.HTML(http.StatusOK, tplAdminHookTypes)
}

// HookTypeNew renders the create form.
func HookTypeNew(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.hook_types.new")
	ctx.Data["PageIsAdminHookTypes"] = true
	ctx.Data["HookType"] = &webhook_model.HookTypeDef{IsActive: true, RequiresPayloadURL: true}
	ctx.HTML(http.StatusOK, tplAdminHookTypeEdit)
}

// HookTypeEdit renders the edit form.
func HookTypeEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.hook_types.edit")
	ctx.Data["PageIsAdminHookTypes"] = true
	ht, err := webhook_model.GetHookTypeByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetHookTypeByID", err)
		return
	}
	ctx.Data["HookType"] = ht
	ctx.HTML(http.StatusOK, tplAdminHookTypeEdit)
}

func parseHookTypeForm(ctx *context.Context, ht *webhook_model.HookTypeDef) {
	ht.Name = strings.TrimSpace(strings.ToLower(ctx.FormString("name")))
	ht.DisplayName = strings.TrimSpace(ctx.FormString("display_name"))
	ht.DocsURL = strings.TrimSpace(ctx.FormString("docs_url"))
	ht.IconAsset = strings.TrimSpace(ctx.FormString("icon_asset"))
	ht.FormSchema = strings.TrimSpace(ctx.FormString("form_schema"))
	ht.PayloadJsonnet = ctx.FormString("payload_jsonnet")
	ht.RequestJsonnet = ctx.FormString("request_jsonnet")
	ht.RequiresPayloadURL = ctx.FormBool("requires_payload_url")
	ht.UseAuthorizationHeader = ctx.FormString("use_authorization_header")
	ht.UseRequestSecret = ctx.FormString("use_request_secret")
	ht.IsActive = ctx.FormBool("is_active")
	if ht.FormSchema == "" {
		ht.FormSchema = "[]"
	}
	if ht.DisplayName == "" {
		ht.DisplayName = ht.Name
	}
}

func validateHookTypeForm(ctx *context.Context, ht *webhook_model.HookTypeDef, isNew bool) bool {
	if isNew && !hookTypeNamePattern.MatchString(ht.Name) {
		ctx.Flash.Error(ctx.Tr("admin.hook_types.invalid_name"))
		return false
	}
	if ht.DisplayName == "" {
		ctx.Flash.Error(ctx.Tr("form.required_field", "Display Name"))
		return false
	}
	if ht.PayloadJsonnet == "" && ht.RequestJsonnet == "" {
		ctx.Flash.Error(ctx.Tr("admin.hook_types.jsonnet_required"))
		return false
	}
	var fields []webhook_service.FormField
	if err := json.Unmarshal([]byte(ht.FormSchema), &fields); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.hook_types.invalid_form_schema"))
		return false
	}
	return true
}

// HookTypeNewPost creates a custom webhook type.
func HookTypeNewPost(ctx *context.Context) {
	ht := &webhook_model.HookTypeDef{IsBuiltin: false}
	parseHookTypeForm(ctx, ht)
	if !validateHookTypeForm(ctx, ht, true) {
		ctx.Data["HookType"] = ht
		ctx.HTML(http.StatusOK, tplAdminHookTypeEdit)
		return
	}
	if err := webhook_model.CreateHookType(ctx, ht); err != nil {
		ctx.ServerError("CreateHookType", err)
		return
	}
	webhook_service.ReloadHookTypeHandler(ht)
	ctx.Flash.Success(ctx.Tr("admin.hook_types.create_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/hook-types")
}

// HookTypeEditPost updates a webhook type.
func HookTypeEditPost(ctx *context.Context) {
	ht, err := webhook_model.GetHookTypeByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetHookTypeByID", err)
		return
	}
	name := ht.Name
	isBuiltin := ht.IsBuiltin
	parseHookTypeForm(ctx, ht)
	ht.Name = name // name is immutable
	ht.IsBuiltin = isBuiltin
	if !validateHookTypeForm(ctx, ht, false) {
		ctx.Data["HookType"] = ht
		ctx.HTML(http.StatusOK, tplAdminHookTypeEdit)
		return
	}
	if err := webhook_model.UpdateHookType(ctx, ht); err != nil {
		ctx.ServerError("UpdateHookType", err)
		return
	}
	webhook_service.ReloadHookTypeHandler(ht)
	ctx.Flash.Success(ctx.Tr("admin.hook_types.update_success"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/hook-types/" + ctx.PathParam("id"))
}

// HookTypeDelete deletes a custom webhook type (or deactivates builtins).
func HookTypeDelete(ctx *context.Context) {
	ht, err := webhook_model.GetHookTypeByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		ctx.Flash.Error(err.Error())
		ctx.JSONRedirect(setting.AppSubURL + "/-/admin/hook-types")
		return
	}
	if ht.IsBuiltin {
		ht.IsActive = false
		if err := webhook_model.UpdateHookType(ctx, ht); err != nil {
			ctx.Flash.Error(err.Error())
		} else {
			_ = webhook_model.DeactivateWebhooksOfType(ctx, ht.Name)
			webhook_service.ReloadHookTypeHandler(ht)
			ctx.Flash.Success(ctx.Tr("admin.hook_types.deactivate_success"))
		}
	} else {
		if err := webhook_model.DeleteHookType(ctx, ht.ID); err != nil {
			ctx.Flash.Error(err.Error())
		} else {
			_ = webhook_model.DeactivateWebhooksOfType(ctx, ht.Name)
			webhook_service.UnregisterHandler(ht.Name)
			webhook_service.SyncWebhookTypesSetting()
			ctx.Flash.Success(ctx.Tr("admin.hook_types.delete_success"))
		}
	}
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/hook-types")
}

// HookTypeReset resets a builtin type from options/webhooks.
func HookTypeReset(ctx *context.Context) {
	ht, err := webhook_model.GetHookTypeByID(ctx, ctx.FormInt64("id"))
	if err != nil || !ht.IsBuiltin {
		ctx.Flash.Error(ctx.Tr("admin.hook_types.reset_failed"))
		ctx.JSONRedirect(setting.AppSubURL + "/-/admin/hook-types")
		return
	}
	if err := webhook_service.ResetBuiltinHookType(ctx, ht.Name); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("admin.hook_types.reset_success"))
	}
	ctx.JSONRedirect(setting.AppSubURL + "/-/admin/hook-types")
}
