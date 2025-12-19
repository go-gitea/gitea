// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

type RenderPluginSetting struct {
	Storage *Storage
}

var RenderPlugin RenderPluginSetting

func loadRenderPluginFrom(rootCfg ConfigProvider) (err error) {
	sec, _ := rootCfg.GetSection("render_plugins")
	RenderPlugin.Storage, err = getStorage(rootCfg, "render-plugins", "", sec)
	return err
}
