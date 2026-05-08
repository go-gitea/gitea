// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	headerRE    = regexp.MustCompile(`^(// (Copyright [^\n]+|All rights reserved\.)\n)*// Copyright \d{4} (The Gogs Authors|The Gitea Authors|Gitea Authors|Gitea)\.( All rights reserved\.)?\n(// (Copyright [^\n]+|All rights reserved\.)\n)*// SPDX-License-Identifier: [\w.-]+`)
	generatedRE = regexp.MustCompile(`(?m)^// (Code|This file is) [Gg]enerated.*DO NOT EDIT`)
)

var skipDirs = map[string]bool{
	".git":         true,
	".venv":        true,
	"node_modules": true,
	"vendor":       true,
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	bad := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, 512))
		f.Close()
		if err != nil {
			return err
		}
		if generatedRE.Match(data) {
			return nil
		}
		if !headerRE.Match(data) {
			fmt.Printf("%s: missing or invalid copyright header\n", path)
			bad++
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if bad > 0 {
		os.Exit(1)
	}
}
