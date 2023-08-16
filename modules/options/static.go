// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package options

import (
	"io/fs"

	"code.gitea.io/gitea/modules/assetfs"
)

var Assets fs.FS

func BuiltinAssets() *assetfs.Layer {
	return assetfs.Bindata("builtin(bindata)", Assets)
}
