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
	"sync"
)

// regexp is based on go-license, excluding README and NOTICE
// https://github.com/google/go-licenses/blob/master/licenses/find.go
var licenseRe = regexp.MustCompile(`^(?i)((UN)?LICEN(S|C)E|COPYING).*$`)

// ignoredNames are LicenseEntry.Name values to exclude from the output.
var ignoredNames = map[string]bool{
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
	Path string `json:"Path"`
	Dir  string `json:"Dir"`
	Main bool   `json:"Main"`
}

type LicenseEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	LicenseText string `json:"licenseText"`
}

// getModules returns all dependency modules with their local directory paths
// using a single go list command.
func getModules(goCmd string) []ModuleInfo {
	cmd := exec.Command(goCmd, "list", "-deps", "-f",
		"{{if .Module}}{{.Module.Path}}\t{{.Module.Dir}}\t{{.Module.Main}}{{end}}", "./...")
	cmd.Stderr = os.Stderr
	// Use GOOS=linux with CGO to ensure we capture all platform-specific
	// dependencies, matching the CI environment.
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run 'go list -deps': %v\n", err)
		os.Exit(1)
	}

	seen := make(map[string]bool)
	var modules []ModuleInfo
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 || seen[parts[0]] {
			continue
		}
		seen[parts[0]] = true
		modules = append(modules, ModuleInfo{
			Path: parts[0],
			Dir:  parts[1],
			Main: parts[2] == "true",
		})
	}
	return modules
}

// findLicenseFiles scans a module's root directory for license files and returns entries.
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

// getTrackedFiles returns the set of files tracked by git, relative to the
// given directory. This is used to exclude gitignored files from license scanning.
func getTrackedFiles(dir string) map[string]bool {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to run 'git ls-files': %v\n", err)
		return nil
	}
	files := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files[filepath.FromSlash(line)] = true
		}
	}
	return files
}

// findSubdirLicenses walks the main module's directory tree to find license
// files in subdirectories (not the root), which indicate bundled code with
// separate copyright attribution.
func findSubdirLicenses(mod ModuleInfo, trackedFiles map[string]bool) []LicenseEntry {
	var entries []LicenseEntry
	err := filepath.WalkDir(mod.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden directories and common non-source directories.
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "internal" || name == "testdata" {
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
		// Skip gitignored files to ensure consistent output across environments.
		if trackedFiles != nil {
			rel, _ := filepath.Rel(mod.Dir, path)
			if !trackedFiles[rel] {
				return nil
			}
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

	// Run git ls-files concurrently with go list.
	var trackedFiles map[string]bool
	var modules []ModuleInfo
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		trackedFiles = getTrackedFiles(".")
	}()
	go func() {
		defer wg.Done()
		modules = getModules(goCmd)
	}()
	wg.Wait()

	var entries []LicenseEntry
	for _, mod := range modules {
		if mod.Main {
			// For the main module, scan subdirectories for license files
			// that indicate bundled third-party code with separate copyright.
			entries = append(entries, findSubdirLicenses(mod, trackedFiles)...)
			continue
		}

		// Scan the module root directory for license files.
		entries = append(entries, findLicenseFiles(mod.Dir, mod.Path)...)
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

	// Ensure file has a final newline.
	if jsonBytes[len(jsonBytes)-1] != '\n' {
		jsonBytes = append(jsonBytes, '\n')
	}

	if err := os.WriteFile(out, jsonBytes, 0o644); err != nil {
		panic(err)
	}
}
