// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// regexp is based on go-license, excluding README
// https://github.com/google/go-licenses/blob/master/licenses/find.go
var licenseRe = regexp.MustCompile(`^(?i)((UN)?LICEN(S|C)E|COPYING|NOTICE).*$`)

type LicenseEntry struct {
	Name        string `json:"name"`
	LicenseText string `json:"licenseText"`
}

func main() {
	base, out := os.Args[1], os.Args[2]

	paths := []string{}
	filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		if entry.IsDir() || !licenseRe.MatchString(entry.Name()) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	sort.Strings(paths)

	entries := []LicenseEntry{}
	for _, path := range paths {
		licenseText, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}

		entries = append(entries, LicenseEntry{
			Name:        filepath.Dir(strings.Replace(path, base+string(os.PathSeparator), "", 1)),
			LicenseText: string(licenseText),
		})
	}

	jsonBytes, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(out, jsonBytes, 0o644)
	if err != nil {
		panic(err)
	}
}
