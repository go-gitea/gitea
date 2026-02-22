// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// regexp is based on go-license, excluding README and NOTICE
// https://github.com/google/go-licenses/blob/master/licenses/find.go
var licenseRe = regexp.MustCompile(`^(?i)((UN)?LICEN(S|C)E|COPYING).*$`)

// primaryLicenseRe matches exact primary license filenames without suffixes.
// When a directory has both primary and variant files (e.g. LICENSE and
// LICENSE.docs), only the primary files are kept.
var primaryLicenseRe = regexp.MustCompile(`^(?i)(LICEN[SC]E|COPYING)$`)

// ignoredNames are LicenseEntry.Name values to exclude from the output.
var ignoredNames = map[string]bool{
	"code.gitea.io/gitea":                 true,
	"code.gitea.io/gitea/options/license": true,
}

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
	Path    string
	Dir     string
	PkgDirs []string // directories of packages imported from this module
}

type LicenseEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	LicenseText string `json:"licenseText"`
}

// getModules returns all dependency modules with their local directory paths
// and the package directories used from each module.
func getModules(goCmd string) []ModuleInfo {
	cmd := exec.Command(goCmd, "list", "-deps", "-f",
		"{{if .Module}}{{.Module.Path}}\t{{.Module.Dir}}\t{{.Dir}}{{end}}", "./...")
	cmd.Stderr = os.Stderr
	// Use GOOS=linux with CGO to ensure we capture all platform-specific
	// dependencies, matching the CI environment.
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run 'go list -deps': %v\n", err)
		os.Exit(1)
	}

	var modules []ModuleInfo
	seen := make(map[string]int) // module path -> index in modules
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}
		modPath, modDir, pkgDir := parts[0], parts[1], parts[2]
		if idx, ok := seen[modPath]; ok {
			modules[idx].PkgDirs = append(modules[idx].PkgDirs, pkgDir)
		} else {
			seen[modPath] = len(modules)
			modules = append(modules, ModuleInfo{
				Path:    modPath,
				Dir:     modDir,
				PkgDirs: []string{pkgDir},
			})
		}
	}
	return modules
}

// findLicenseFiles scans a module's root directory and its used package
// directories for license files. It also walks up from each package directory
// to the module root, scanning intermediate directories. Subdirectory licenses
// are only included if their text differs from the root license(s).
func findLicenseFiles(mod ModuleInfo) []LicenseEntry {
	var entries []LicenseEntry
	seenTexts := make(map[string]bool)

	// First, collect root-level license files.
	entries = append(entries, scanDirForLicenses(mod.Dir, mod.Path, "")...)
	for _, e := range entries {
		seenTexts[e.LicenseText] = true
	}

	// Then check each package directory and all intermediate parent directories
	// up to the module root for license files with unique text.
	seenDirs := map[string]bool{mod.Dir: true}
	for _, pkgDir := range mod.PkgDirs {
		for dir := pkgDir; dir != mod.Dir && strings.HasPrefix(dir, mod.Dir); dir = filepath.Dir(dir) {
			if seenDirs[dir] {
				continue
			}
			seenDirs[dir] = true
			for _, e := range scanDirForLicenses(dir, mod.Path, mod.Dir) {
				if !seenTexts[e.LicenseText] {
					seenTexts[e.LicenseText] = true
					entries = append(entries, e)
				}
			}
		}
	}
	return entries
}

// scanDirForLicenses reads a single directory for license files and returns entries.
// If moduleRoot is non-empty, paths are made relative to it.
func scanDirForLicenses(dir, modulePath, moduleRoot string) []LicenseEntry {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
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
			continue
		}

		entryName := modulePath
		entryPath := modulePath + "/" + name
		if moduleRoot != "" {
			rel, _ := filepath.Rel(moduleRoot, dir)
			if rel != "." {
				relSlash := filepath.ToSlash(rel)
				entryName = modulePath + "/" + relSlash
				entryPath = modulePath + "/" + relSlash + "/" + name
			}
		}

		entries = append(entries, LicenseEntry{
			Name:        entryName,
			Path:        entryPath,
			LicenseText: string(content),
		})
	}

	// When multiple license files exist, prefer primary files (e.g. LICENSE)
	// over variants with suffixes (e.g. LICENSE.docs, LICENSE-2.0.txt).
	// If no primary file exists, keep only the first variant.
	if len(entries) > 1 {
		var primary []LicenseEntry
		for _, e := range entries {
			fileName := e.Path[strings.LastIndex(e.Path, "/")+1:]
			if primaryLicenseRe.MatchString(fileName) {
				primary = append(primary, e)
			}
		}
		if len(primary) > 0 {
			return primary
		}
		return entries[:1]
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

	modules := getModules(goCmd)

	var entries []LicenseEntry
	for _, mod := range modules {
		entries = append(entries, findLicenseFiles(mod)...)
	}

	entries = slices.DeleteFunc(entries, func(e LicenseEntry) bool {
		return ignoredNames[e.Name]
	})

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	jsonBytes, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		panic(err)
	}

	// Ensure file has a final newline
	if jsonBytes[len(jsonBytes)-1] != '\n' {
		jsonBytes = append(jsonBytes, '\n')
	}

	err = os.WriteFile(out, jsonBytes, 0o644)
	if err != nil {
		panic(err)
	}
}
