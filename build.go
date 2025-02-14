// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build vendor

package main

// Libraries that are included to vendor utilities used during build.
// These libraries will not be included in a normal compilation.

import (
	// for embed
	_ "github.com/shurcooL/vfsgen"

	// for cover merge
	_ "golang.org/x/tools/cover"

	// for vet
	_ "code.gitea.io/gitea-vet"

	// for swagger
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
)
