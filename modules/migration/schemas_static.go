// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build bindata

package migration

import (
	"io"
	"path"
)

func openSchema(filename string) (io.ReadCloser, error) {
	return Assets.Open(path.Base(filename))
}
