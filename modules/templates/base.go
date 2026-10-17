// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"gitea.dev/modules/assetfs"
	"gitea.dev/modules/setting"
)

func AssetFS() *assetfs.LayeredFS {
	return assetfs.Layered(CustomAssets(), BuiltinAssets())
}

func CustomAssets() *assetfs.Layer {
	return assetfs.Local("custom", setting.CustomPath, "templates")
}
