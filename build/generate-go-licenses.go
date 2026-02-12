// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	".yaml":      true,
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

// getDepModules returns the set of module paths that are actual dependencies
// (imported by non-test code), excluding tool and test-only transitive deps.
func getDepModules(goCmd string) map[string]bool {
	cmd := exec.Command(goCmd, "list", "-deps", "-f", "{{if .Module}}{{.Module.Path}}{{end}}", "./...")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run 'go list -deps': %v\n", err)
		os.Exit(1)
	}

	modules := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			modules[line] = true
		}
	}
	return modules
}

// findLicenseFiles scans a directory for license files and returns entries.
func findLicenseFiles(dir, modulePath string) []LicenseEntry {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to read directory %s for %s: %v\n", dir, modulePath, err)
		return nil
	}

	var entries []LicenseEntry
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

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to read %s/%s: %v\n", modulePath, name, err)
			continue
		}

		entries = append(entries, LicenseEntry{
			Name:        modulePath,
			Path:        modulePath + "/" + name,
			LicenseText: string(content),
		})
	}
	return entries
}

// findSubdirLicenses walks the main module's directory tree to find license
// files in subdirectories (not the root), which indicate bundled code with
// separate copyright attribution.
func findSubdirLicenses(mod ModuleInfo) []LicenseEntry {
	var entries []LicenseEntry
	err := filepath.WalkDir(mod.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden directories and common non-source directories.
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip files in the root directory (that's the main module's own license).
		if filepath.Dir(path) == mod.Dir {
			return nil
		}
		if !licenseRe.MatchString(d.Name()) {
			return nil
		}
		if excludedExt[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(mod.Dir, path)
		entryPath := mod.Path + "/" + filepath.ToSlash(rel)
		entryName := mod.Path + "/" + filepath.ToSlash(filepath.Dir(rel))
		entries = append(entries, LicenseEntry{
			Name:        entryName,
			Path:        entryPath,
			LicenseText: string(content),
		})
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to walk main module directory: %v\n", err)
	}
	return entries
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: go run generate-go-licenses.go <out-json-file>")
		os.Exit(1)
	}

	out := os.Args[1]

	goCmd := "go"
	if env := os.Getenv("GO"); env != "" {
		goCmd = env
	}

	// Get the set of modules that are actual non-test dependencies,
	// excluding tool dependencies and test-only transitive deps.
	depModules := getDepModules(goCmd)

	// Get all modules with their local directory paths from the module cache.
	cmd := exec.Command(goCmd, "list", "-m", "-json", "all")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run 'go list -m -json all': %v\n", err)
		os.Exit(1)
	}

	// Parse streaming JSON (multiple JSON objects concatenated).
	var modules []ModuleInfo
	dec := json.NewDecoder(bytes.NewReader(output))
	for {
		var mod ModuleInfo
		if err := dec.Decode(&mod); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "failed to decode module info: %v\n", err)
			os.Exit(1)
		}
		modules = append(modules, mod)
	}

	var entries []LicenseEntry
	for _, mod := range modules {
		if mod.Dir == "" {
			continue
		}

		if mod.Main {
			// For the main module, scan subdirectories for license files
			// that indicate bundled third-party code with separate copyright.
			entries = append(entries, findSubdirLicenses(mod)...)
			continue
		}

		// Skip modules that are not actual dependencies (tool deps, test-only transitive deps).
		if !depModules[mod.Path] {
			continue
		}

		// Scan the module root directory for license files.
		entries = append(entries, findLicenseFiles(mod.Dir, mod.Path)...)
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
