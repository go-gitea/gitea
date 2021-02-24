// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//+build vendor

package main

// Libraries that are included to vendor utilities used during build.
// These libraries will not be included in a normal compilation.

import (
	// for lint
	_ "github.com/mgechev/dots"
	_ "github.com/mgechev/revive/formatter"
	_ "github.com/mgechev/revive/lint"
	_ "github.com/mgechev/revive/rule"
	_ "github.com/mitchellh/go-homedir"
	_ "github.com/pelletier/go-toml"

	// for embed
	_ "github.com/shurcooL/vfsgen"

	// for cover merge
	_ "golang.org/x/tools/cover"

	// for vet
	_ "code.gitea.io/gitea-vet"

	// for swagger
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
)
