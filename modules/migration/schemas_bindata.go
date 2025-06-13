// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

//go:generate go run ../../build/generate-bindata.go ../../modules/migration/schemas bindata.dat

package migration

import (
	"io"
	"io/fs"
	"path"
	"sync"

	_ "embed"

	"code.gitea.io/gitea/modules/assetfs"
)

//go:embed bindata.dat
var bindata []byte

var BuiltinAssets = sync.OnceValue(func() fs.FS {
	return assetfs.NewEmbeddedFS(bindata)
})

func openSchema(filename string) (io.ReadCloser, error) {
	return BuiltinAssets().Open(path.Base(filename))
}
