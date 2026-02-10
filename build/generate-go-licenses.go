// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// regexp is based on go-license, excluding README and NOTICE
// https://github.com/google/go-licenses/blob/master/licenses/find.go
var licenseRe = regexp.MustCompile(`^(?i)((UN)?LICEN(S|C)E|COPYING).*$`)

var excludedExt = map[string]bool{
	".gitignore": true,
	".go":        true,
	".mod":       true,
	".sum":       true,
	".toml":      true,
	".yml":       true,
}

type ModuleInfo struct {
	Path string `json:"Path"`
	Dir  string `json:"Dir"`
	Main bool   `json:"Main"`
}

type LicenseEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	LicenseText string `json:"licenseText"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: go run generate-go-licenses.go <out-json-file>")
		os.Exit(1)
	}

	out := os.Args[1]

	// Get all modules with their local directory paths from the module cache.
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run 'go list -m -json all': %v\n", err)
		os.Exit(1)
	}

	// Parse streaming JSON (multiple JSON objects concatenated).
	var modules []ModuleInfo
	dec := json.NewDecoder(bytes.NewReader(output))
	for dec.More() {
		var mod ModuleInfo
		if err := dec.Decode(&mod); err != nil {
			fmt.Fprintf(os.Stderr, "failed to decode module info: %v\n", err)
			os.Exit(1)
		}
		modules = append(modules, mod)
	}

	var entries []LicenseEntry
	for _, mod := range modules {
		// Skip the main module and modules without a local directory.
		if mod.Main || mod.Dir == "" {
			continue
		}

		// Scan the module root directory for license files.
		dirEntries, err := os.ReadDir(mod.Dir)
		if err != nil {
			continue
		}

		for _, entry := range dirEntries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !licenseRe.MatchString(name) {
				continue
			}
			if excludedExt[strings.ToLower(filepath.Ext(name))] {
				continue
			}

			content, err := os.ReadFile(filepath.Join(mod.Dir, name))
			if err != nil {
				continue
			}

			entries = append(entries, LicenseEntry{
				Name:        mod.Path,
				Path:        mod.Path + "/" + name,
				LicenseText: string(content),
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	jsonBytes, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		panic(err)
	}

	// Ensure file has a final newline.
	if jsonBytes[len(jsonBytes)-1] != '\n' {
		jsonBytes = append(jsonBytes, '\n')
	}

	if err := os.WriteFile(out, jsonBytes, 0o644); err != nil {
		panic(err)
	}
}
