// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

//go:generate go run ../../build/generate-bindata.go ../../modules/migration/schemas bindata.dat

package migration

import (
	"io/fs"
	"path"
	"sync"

	_ "embed"

	"code.gitea.io/gitea/modules/assetfs"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed bindata.dat
var bindata []byte

var BuiltinAssets = sync.OnceValue(func() fs.FS {
	return assetfs.NewEmbeddedFS(bindata)
})

func openSchema(filename string) (any, error) {
	f, err := BuiltinAssets().Open(path.Base(filename))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return jsonschema.UnmarshalJSON(f)
}
