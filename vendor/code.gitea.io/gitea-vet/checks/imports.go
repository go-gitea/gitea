// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package checks

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Imports = &analysis.Analyzer{
	Name: "imports",
	Doc:  "check for import order",
	Run:  runImports,
}

func runImports(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		level := 0
		for _, im := range file.Imports {
			var lvl int
			val := im.Path.Value
			switch {
			case importHasPrefix(val, "code.gitea.io"):
				lvl = 2
			case strings.Contains(val, "."):
				lvl = 3
			default:
				lvl = 1
			}

			if lvl < level {
				pass.Reportf(file.Pos(), "Imports are sorted wrong")
				break
			}
			level = lvl
		}
	}
	return nil, nil
}

func importHasPrefix(s, p string) bool {
	return strings.HasPrefix(s, "\""+p)
}
