// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"

	render_model "code.gitea.io/gitea/models/render"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	plugin_service "code.gitea.io/gitea/services/renderplugin"
)

const (
	tplRenderPlugins       templates.TplName = "admin/render/plugins"
	tplRenderPluginDetail  templates.TplName = "admin/render/plugin_detail"
	tplRenderPluginConfirm templates.TplName = "admin/render/plugin_confirm"
)

type pendingRenderPluginUpload struct {
	Path               string
	Filename           string
	ExpectedIdentifier string
	PluginID           int64
}

var (
	pendingUploadsMu sync.Mutex
	pendingUploads   = make(map[string]*pendingRenderPluginUpload)
)

func rememberPendingUpload(info *pendingRenderPluginUpload) (string, error) {
	for {
		token, err := util.CryptoRandomString(32)
		if err != nil {
			return "", err
		}
		pendingUploadsMu.Lock()
		if _, ok := pendingUploads[token]; ok {
			pendingUploadsMu.Unlock()
			continue
		}
		pendingUploads[token] = info
		pendingUploadsMu.Unlock()
		return token, nil
	}
}

func takePendingUpload(token string) *pendingRenderPluginUpload {
	if token == "" {
		return nil
	}
	pendingUploadsMu.Lock()
	defer pendingUploadsMu.Unlock()
	info := pendingUploads[token]
	delete(pendingUploads, token)
	return info
}

func discardPendingUpload(info *pendingRenderPluginUpload) {
	if info == nil {
		return
	}
	if err := os.Remove(info.Path); err != nil && !os.IsNotExist(err) {
		log.Warn("Failed to remove pending render plugin upload %s: %v", info.Path, err)
	}
}

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
	previewPath, err := saveRenderPluginUpload(file)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	manifest, err := plugin_service.LoadManifestFromArchive(previewPath)
	if err != nil {
		_ = os.Remove(previewPath)
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	token, err := rememberPendingUpload(&pendingRenderPluginUpload{
		Path:               previewPath,
		Filename:           header.Filename,
		ExpectedIdentifier: "",
		PluginID:           0,
	})
	if err != nil {
		_ = os.Remove(previewPath)
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	ctx.Data["Title"] = ctx.Tr("admin.render_plugins.confirm_install", manifest.Name)
	ctx.Data["PageIsAdminRenderPlugins"] = true
	ctx.Data["PluginManifest"] = manifest
	ctx.Data["UploadFilename"] = header.Filename
	ctx.Data["PendingUploadToken"] = token
	ctx.Data["IsUpgradePreview"] = false
	ctx.Data["RedirectTo"] = ctx.FormString("redirect_to")
	ctx.HTML(http.StatusOK, tplRenderPluginConfirm)
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
	previewPath, err := saveRenderPluginUpload(file)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upgrade_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	manifest, err := plugin_service.LoadManifestFromArchive(previewPath)
	if err != nil {
		_ = os.Remove(previewPath)
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upgrade_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	if manifest.ID != plug.Identifier {
		_ = os.Remove(previewPath)
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.identifier_mismatch", manifest.ID, plug.Identifier))
		redirectRenderPlugins(ctx)
		return
	}
	token, err := rememberPendingUpload(&pendingRenderPluginUpload{
		Path:               previewPath,
		Filename:           header.Filename,
		ExpectedIdentifier: plug.Identifier,
		PluginID:           plug.ID,
	})
	if err != nil {
		_ = os.Remove(previewPath)
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upgrade_failed", err))
		redirectRenderPlugins(ctx)
		return
	}
	ctx.Data["Title"] = ctx.Tr("admin.render_plugins.confirm_upgrade", plug.Name)
	ctx.Data["PageIsAdminRenderPlugins"] = true
	ctx.Data["PluginManifest"] = manifest
	ctx.Data["UploadFilename"] = header.Filename
	ctx.Data["PendingUploadToken"] = token
	ctx.Data["IsUpgradePreview"] = true
	ctx.Data["CurrentPlugin"] = plug
	ctx.Data["RedirectTo"] = ctx.FormString("redirect_to")
	ctx.HTML(http.StatusOK, tplRenderPluginConfirm)
}

// RenderPluginsUploadConfirm finalizes a pending plugin installation.
func RenderPluginsUploadConfirm(ctx *context.Context) {
	info := takePendingUpload(ctx.FormString("token"))
	if info == nil || info.PluginID != 0 {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_token_invalid"))
		if info != nil {
			discardPendingUpload(info)
		}
		redirectRenderPlugins(ctx)
		return
	}
	_, err := installPendingUpload(ctx, info)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_failed", err))
	} else {
		ctx.Flash.Success(ctx.Tr("admin.render_plugins.upload_success", info.Filename))
	}
	redirectRenderPlugins(ctx)
}

// RenderPluginsUpgradeConfirm finalizes a pending plugin upgrade.
func RenderPluginsUpgradeConfirm(ctx *context.Context) {
	plug := mustGetRenderPlugin(ctx)
	if plug == nil {
		return
	}
	info := takePendingUpload(ctx.FormString("token"))
	if info == nil || info.PluginID != plug.ID || info.ExpectedIdentifier != plug.Identifier {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upload_token_invalid"))
		if info != nil {
			discardPendingUpload(info)
		}
		redirectRenderPlugins(ctx)
		return
	}
	updated, err := installPendingUpload(ctx, info)
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.render_plugins.upgrade_failed", err))
	} else {
		ctx.Flash.Success(ctx.Tr("admin.render_plugins.upgrade_success", updated.Name, updated.Version))
	}
	redirectRenderPlugins(ctx)
}

// RenderPluginsUploadDiscard removes a pending upload archive without installing it.
func RenderPluginsUploadDiscard(ctx *context.Context) {
	info := takePendingUpload(ctx.FormString("token"))
	if info != nil {
		discardPendingUpload(info)
	}
	ctx.Flash.Success(ctx.Tr("admin.render_plugins.upload_discarded"))
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

func saveRenderPluginUpload(file multipart.File) (_ string, err error) {
	tmpFile, cleanup, err := setting.AppDataTempDir("render-plugins").CreateTempFileRandom("pending", "*.zip")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	if _, err = io.Copy(tmpFile, file); err != nil {
		return "", err
	}
	if err = tmpFile.Close(); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func installPendingUpload(ctx *context.Context, info *pendingRenderPluginUpload) (*render_model.Plugin, error) {
	file, err := os.Open(info.Path)
	if err != nil {
		discardPendingUpload(info)
		return nil, err
	}
	defer file.Close()
	defer discardPendingUpload(info)
	return plugin_service.InstallFromArchive(ctx, file, info.Filename, info.ExpectedIdentifier)
}
