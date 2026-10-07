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
	tplAdminDevContainerTemplates templates.TplName = "codespace/admin_devcontainer_templates"
	tplUserDevContainerTemplates  templates.TplName = "codespace/user_devcontainer_templates"
)

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
