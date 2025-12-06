// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"net/http"
	"strings"

	render_model "code.gitea.io/gitea/models/render"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	plugin_service "code.gitea.io/gitea/services/renderplugin"
)

const (
	tplRenderPlugins      templates.TplName = "admin/render/plugins"
	tplRenderPluginDetail templates.TplName = "admin/render/plugin_detail"
)

// RenderPlugins shows the plugin management page.
func RenderPlugins(ctx *context.Context) {
	plugs, err := render_model.ListPlugins(ctx)
	if err != nil {
		ctx.ServerError("ListPlugins", err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("admin.render_plugins")
	ctx.Data["PageIsAdminRenderPlugins"] = true
	ctx.Data["Plugins"] = plugs
	ctx.HTML(http.StatusOK, tplRenderPlugins)
}

// RenderPluginDetail shows a single plugin detail page.
func RenderPluginDetail(ctx *context.Context) {
	plug := mustGetRenderPlugin(ctx)
	if plug == nil {
		return
	}
	ctx.Data["Title"] = ctx.Tr("admin.render_plugins.detail_title", plug.Name)
	ctx.Data["PageIsAdminRenderPlugins"] = true
	ctx.Data["Plugin"] = plug
	ctx.HTML(http.StatusOK, tplRenderPluginDetail)
}

// RenderPluginsUpload handles plugin uploads.
func RenderPluginsUpload(ctx *context.Context) {
	file, header, err := ctx.Req.FormFile("plugin")
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	defer file.Close()
	if header.Size == 0 {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_missing"))
		redirectRenderPlugins(ctx)
		return
	}
	if _, err := plugin_service.InstallFromArchive(ctx, file, header.Filename, ""); err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_failed", err))
	} else {
		ctx.Flash.Success(ctx.Tr("admin.render_plugins.upload_success", header.Filename))
	}
	redirectRenderPlugins(ctx)
}

// RenderPluginsEnable toggles plugin state to enabled.
func RenderPluginsEnable(ctx *context.Context) {
	plug := mustGetRenderPlugin(ctx)
	if plug == nil {
		return
	}
	if err := plugin_service.SetEnabled(ctx, plug, true); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("admin.render_plugins.enabled", plug.Name))
	}
	redirectRenderPlugins(ctx)
}

// RenderPluginsDisable toggles plugin state to disabled.
func RenderPluginsDisable(ctx *context.Context) {
	plug := mustGetRenderPlugin(ctx)
	if plug == nil {
		return
	}
	if err := plugin_service.SetEnabled(ctx, plug, false); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("admin.render_plugins.disabled", plug.Name))
	}
	redirectRenderPlugins(ctx)
}

// RenderPluginsDelete removes a plugin entirely.
func RenderPluginsDelete(ctx *context.Context) {
	plug := mustGetRenderPlugin(ctx)
	if plug == nil {
		return
	}
	if err := plugin_service.Delete(ctx, plug); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("admin.render_plugins.deleted", plug.Name))
	}
	redirectRenderPlugins(ctx)
}

// RenderPluginsUpgrade upgrades an existing plugin with a new archive.
func RenderPluginsUpgrade(ctx *context.Context) {
	plug := mustGetRenderPlugin(ctx)
	if plug == nil {
		return
	}
	file, header, err := ctx.Req.FormFile("plugin")
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upgrade_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	defer file.Close()
	if header.Size == 0 {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_missing"))
		redirectRenderPlugins(ctx)
		return
	}
	updated, err := plugin_service.InstallFromArchive(ctx, file, header.Filename, plug.Identifier)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upgrade_failed", err))
	} else {
		ctx.Flash.Success(ctx.Tr("admin.render_plugins.upgrade_success", updated.Name, updated.Version))
	}
	redirectRenderPlugins(ctx)
}

func mustGetRenderPlugin(ctx *context.Context) *render_model.Plugin {
	id := ctx.PathParamInt64("id")
	if id <= 0 {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.invalid"))
		redirectRenderPlugins(ctx)
		return nil
	}
	plug, err := render_model.GetPluginByID(ctx, id)
	if err != nil {
		ctx.Flash.Error(fmt.Sprintf("%v", err))
		redirectRenderPlugins(ctx)
		return nil
	}
	return plug
}

func redirectRenderPlugins(ctx *context.Context) {
	redirectTo := ctx.FormString("redirect_to")
	if redirectTo != "" {
		base := setting.AppSubURL + "/"
		if strings.HasPrefix(redirectTo, base) {
			ctx.Redirect(redirectTo)
			return
		}
	}
	ctx.Redirect(setting.AppSubURL + "/-/admin/render-plugins")
}
