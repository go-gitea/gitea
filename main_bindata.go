// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package main

import (
	"embed"
	"io/fs"

	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/templates"
)

//go:embed options public templates modules/migration/schemas
var bindata embed.FS

func init() {
	migration.Assets, _ = fs.Sub(bindata, "modules/migration/schemas")
	options.Assets, _ = fs.Sub(bindata, "options")
	public.Assets, _ = fs.Sub(bindata, "public")
	templates.Assets, _ = fs.Sub(bindata, "templates")
}
