// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// regexp is based on go-license, excluding README and NOTICE
// https://github.com/google/go-licenses/blob/master/licenses/find.go
var licenseRe = regexp.MustCompile(`^(?i)((UN)?LICEN(S|C)E|COPYING).*$`)

type LicenseEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	LicenseText string `json:"licenseText"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: go run generate-go-licenses.go <base-dir> <out-json-file>")
		os.Exit(1)
	}

	base, out := os.Args[1], os.Args[2]

	// Add ext for excluded files because license_test.go will be included for some reason.
	// And there are more files that should be excluded, check with:
	//
	// go run github.com/google/go-licenses@v1.6.0 save . --force --save_path=.go-licenses 2>/dev/null
	// find .go-licenses -type f | while read FILE; do echo "${$(basename $FILE)##*.}"; done | sort -u
	//    AUTHORS
	//    COPYING
	//    LICENSE
	//    Makefile
	//    NOTICE
	//    gitignore
	//    go
	//    md
	//    mod
	//    sum
	//    toml
	//    txt
	//    yml
	//
	// It could be removed once we have a better regex.
	excludedExt := map[string]bool{
		".gitignore": true,
		".go":        true,
		".mod":       true,
		".sum":       true,
		".toml":      true,
		".yml":       true,
	}
	var paths []string
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !licenseRe.MatchString(entry.Name()) || excludedExt[filepath.Ext(entry.Name())] {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		panic(err)
	}

	sort.Strings(paths)

	var entries []LicenseEntry
	for _, filePath := range paths {
		licenseText, err := os.ReadFile(filePath)
		if err != nil {
			panic(err)
		}

		pkgPath := filepath.ToSlash(filePath)
		pkgPath = strings.TrimPrefix(pkgPath, base+"/")
		pkgName := path.Dir(pkgPath)

		// There might be a bug somewhere in go-licenses that sometimes interprets the
		// root package as "." and sometimes as "code.gitea.io/gitea". Workaround by
		// removing both of them for the sake of stable output.
		if pkgName == "." || pkgName == "code.gitea.io/gitea" {
			continue
		}

		entries = append(entries, LicenseEntry{
			Name:        pkgName,
			Path:        pkgPath,
			LicenseText: string(licenseText),
		})
	}

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
