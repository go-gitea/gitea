// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build ignore

// Gitea's code formatter:
// * Sort imports with 3 groups: std, gitea, other

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/build/codeformat"
)

func showUsage() {
	fmt.Printf("Usage: codeformat {-l|-w} directory\n")
}

var ignoreList = []string{
	"_bindata.go",
	"tests/gitea-repositories-meta",
	"tests/integration/migration-test",
	"modules/git/tests",
	"models/fixtures",
	"models/migrations/fixtures",
	"services/gitdiff/testdata",
}

func main() {
	if len(os.Args) != 3 {
		showUsage()
		os.Exit(1)
	}
	doList := os.Args[1] == "-l"
	doWrite := os.Args[1] == "-w"
	dir := os.Args[2]
	if !doList && !doWrite {
		showUsage()
		fmt.Printf("You should set either '-l' or '-w'\n")
		os.Exit(1)
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		path = strings.ReplaceAll(path, "\\", "/")
		for _, ignore := range ignoreList {
			if strings.Contains(path, ignore) {
				return filepath.SkipDir
			}
		}
		if d.IsDir() {
			return nil // walk into
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if err := codeformat.FormatGoImports(path, doList, doWrite); err != nil {
			log.Printf("Failed to format go imports: %s, err=%v", path, err)
			return err
		}
		return nil
	})
	if err != nil {
		log.Printf("Failed to format code by walking directory: %s, err=%v", dir, err)
		os.Exit(1)
	}
}
