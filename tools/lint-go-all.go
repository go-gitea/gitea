// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/modfile"
)

// goModIgnoredDirs returns the go.mod "ignore" directories, which the go tool skips but a filesystem walk does not.
func goModIgnoredDirs() (map[string]bool, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return nil, err
	}
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}
	dirs := make(map[string]bool, len(mod.Ignore))
	for _, ignore := range mod.Ignore {
		dirs[filepath.ToSlash(filepath.Clean(ignore.Path))] = true
	}
	return dirs, nil
}

func lintGoHeader() bool {
	headerRE := regexp.MustCompile(`^(// (Copyright [^\n]+|All rights reserved\.)\n)*// Copyright \d{4} (The Gogs Authors|The Gitea Authors|Gitea Authors|Gitea)\.( All rights reserved\.)?\n(// (Copyright [^\n]+|All rights reserved\.)\n)*// SPDX-License-Identifier: [\w.-]+`)
	generatedRE := regexp.MustCompile(`(?m)^// (Code|This file is) [Gg]enerated.*DO NOT EDIT`)
	skipDirs, err := goModIgnoredDirs()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return false
	}
	root, bad := ".", 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if skipDirs[filepath.ToSlash(path)] || strings.HasPrefix(d.Name(), ".") {
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
		_ = f.Close()
		if err != nil {
			return err
		}
		if generatedRE.Match(data) {
			return nil
		}
		if !headerRE.Match(data) {
			_, _ = fmt.Fprintf(os.Stderr, "%s: missing or invalid copyright header\n", path)
			bad++
		}
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
	return err == nil && bad == 0
}

func runCmd(env []string, name string, args []string) bool {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return false
	}
	return true
}

func main() {
	// 'go run' can not have distinct GOOS/GOARCH for its build and run steps,
	// so install a pre-compiled binary and run it for different target platforms.
	_, _ = os.Unsetenv("GOOS"), os.Unsetenv("GOARCH")

	envGolangciLintPackage := os.Getenv("GOLANGCI_LINT_PACKAGE")
	envGo := os.Getenv("GO")
	if envGo == "" || envGolangciLintPackage == "" {
		_, _ = fmt.Fprintln(os.Stderr, "Environment variables GO and GOLANGCI_LINT_PACKAGE must be set")
		os.Exit(1)
	}
	if !runCmd(nil, envGo, []string{"install", envGolangciLintPackage}) {
		os.Exit(1)
	}

	_, _ = fmt.Fprintln(os.Stdout, "lint go header ...")
	succeed := lintGoHeader()

	_, _ = fmt.Fprintln(os.Stdout, "lint for linux ...")
	lintTagsLinux := ""
	if os.Getenv("CI") != "" || strings.Contains(os.Getenv("TAGS"), "bindata") {
		// also lint with bindata tag if we are in CI or the "bindata" is explicitly set in the env TAGS
		lintTagsLinux = "bindata"
	}
	succeed = runCmd([]string{"GOOS=linux", "TAGS=" + lintTagsLinux}, "golangci-lint", append([]string{"run", "--build-tags=linux," + lintTagsLinux}, os.Args[1:]...)) && succeed

	if os.Getenv("CI") != "" {
		// only lint for other platforms when in CI, to keep local lint fast
		_, _ = fmt.Fprintln(os.Stdout, "lint for windows ...")
		succeed = runCmd([]string{"GOOS=windows", "TAGS=gogit"}, "golangci-lint", append([]string{"run", "--build-tags=windows,gogit"}, os.Args[1:]...)) && succeed
	}

	if !succeed {
		os.Exit(1)
	}
}
