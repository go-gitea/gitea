// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
)

var svgUsageRE = regexp.MustCompile("svg [\"'`]([^\"'`]+)[\"'`]")

func main() {
	if err := checkSVGReferences(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	customPath, err := os.MkdirTemp("", "gitea-lint-templates-custom-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir for custom templates: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(customPath)

	setting.StaticRootPath = "."
	setting.CustomPath = customPath
	setting.IsProd = true // avoid the dev-mode file watcher while compiling
	if err := templates.ReloadAllTemplates(); err != nil {
		fmt.Fprintf(os.Stderr, "template compilation failed: %v\n", err)
		os.Exit(1)
	}
}

func checkSVGReferences() error {
	knownSVGs := map[string]bool{}
	entries, err := os.ReadDir("public/assets/img/svg")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		knownSVGs[strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))] = true
	}

	hadErrors := false
	err = filepath.WalkDir("templates", func(path string, entry os.DirEntry, walkErr error) error {
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
		for _, match := range svgUsageRE.FindAllStringSubmatch(string(content), -1) {
			if !knownSVGs[match[1]] {
				fmt.Printf("SVG %q not found, used in %s\n", match[1], path)
				hadErrors = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if hadErrors {
		return fmt.Errorf("unknown SVG references found in templates")
	}
	return nil
}
