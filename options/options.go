// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic
// +build !servedynamic

package options

import (
	"embed"
	"io/fs"
)

// OptionsFS contains the options definitions.
//
//go:embed gitignore label license locale readme
var OptionsFS embed.FS

func Asset(name string) ([]byte, error) {
	return fs.ReadFile(OptionsFS, name)
}

func AssetNames() []string {
	var results []string
	_ = fs.WalkDir(OptionsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if d.Type().IsRegular() {
			results = append(results, path)
		}
		return nil
	})
	return results
}

func AssetDir(dirName string) ([]string, error) {
	files, err := fs.ReadDir(OptionsFS, dirName)
	if err != nil {
		return nil, err
	}
	results := make([]string, 0, len(files))
	for _, file := range files {
		results = append(results, file.Name())
	}
	return results, nil
}
