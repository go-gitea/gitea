// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic

package migration

import (
	"io"
	"path"

	"code.gitea.io/gitea/modules/migration/schemas"
)

func openSchema(filename string) (io.ReadCloser, error) {
	return schemas.SchemasFS.Open(path.Base(filename))
}
