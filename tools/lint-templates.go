// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
)

var conflictMarkerRE = regexp.MustCompile(`^(<{7}|={7}|>{7})(\s|$)`)

func main() {
	if err := checkConflictMarkers("templates"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	customPath, err := os.MkdirTemp("", "gitea-lint-templates-custom-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir for custom templates: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(customPath)

	// Compile all templates exactly as the server does, so PRs fail before startup.
	setting.StaticRootPath = "."
	setting.CustomPath = customPath
	setting.IsProd = true // avoid the dev-mode file watcher while compiling
	if err := templates.ReloadAllTemplates(); err != nil {
		fmt.Fprintf(os.Stderr, "template compilation failed: %v\n", err)
		os.Exit(1)
	}
}

func checkConflictMarkers(root string) error {
	foundConflictMarker := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(bytes.NewReader(content))
		for line := 1; scanner.Scan(); line++ {
			if conflictMarkerRE.Match(scanner.Bytes()) {
				fmt.Printf("%s:%d: unresolved conflict marker\n", path, line)
				foundConflictMarker = true
			}
		}
		return scanner.Err()
	})
	if err != nil {
		return err
	}
	if foundConflictMarker {
		return fmt.Errorf("unresolved conflict markers found in templates")
	}
	return nil
}
