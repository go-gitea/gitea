// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"code.gitea.io/gitea-vet/checks"

	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	unitchecker.Main(
		checks.Imports,
		checks.License,
		checks.Migrations,
	)
}
