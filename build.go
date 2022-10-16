// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build vendor

package main

// Libraries that are included to vendor utilities used during build.
// These libraries will not be included in a normal compilation.

import (
	_ "code.gitea.io/gitea-vet"                      // for vet
	_ "github.com/go-swagger/go-swagger/cmd/swagger" // for swagger
	_ "github.com/shurcooL/vfsgen"                   // for embed
	_ "golang.org/x/tools/cover"                     // for cover merge
)
