// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package options

import (
	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/setting"
)

func CustomAssets() *assetfs.Layer {
	return assetfs.Local("custom", setting.CustomPath, "options")
}

func AssetFS() *assetfs.LayeredFS {
	return assetfs.Layered(CustomAssets(), BuiltinAssets())
}

// Locale reads the content of a specific locale from static/bindata or custom path.
func Locale(name string) ([]byte, error) {
	return AssetFS().ReadFile("locale", name)
}

// Readme reads the content of a specific readme from static/bindata or custom path.
func Readme(name string) ([]byte, error) {
	return AssetFS().ReadFile("readme", name)
}

// Gitignore reads the content of a gitignore locale from static/bindata or custom path.
func Gitignore(name string) ([]byte, error) {
	return AssetFS().ReadFile("gitignore", name)
}

// License reads the content of a specific license from static/bindata or custom path.
func License(name string) ([]byte, error) {
	return AssetFS().ReadFile("license", name)
}

// Labels reads the content of a specific labels from static/bindata or custom path.
func Labels(name string) ([]byte, error) {
	return AssetFS().ReadFile("label", name)
}
