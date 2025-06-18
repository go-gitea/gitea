// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

//go:generate go run ../../build/generate-bindata.go ../../templates bindata.dat

package templates

import (
	"sync"

	_ "embed"

	"code.gitea.io/gitea/modules/assetfs"
)

//go:embed bindata.dat
var bindata []byte

var BuiltinAssets = sync.OnceValue(func() *assetfs.Layer {
	return assetfs.Bindata("builtin(bindata)", assetfs.NewEmbeddedFS(bindata))
})
