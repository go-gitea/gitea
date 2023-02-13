// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic

package templates

import (
	"embed"
	"io/fs"
)

// TemplatesFS contains the embedded templates.
//
//go:embed admin api base code custom explore mail swagger
//go:embed org package projects repo shared status user
//go:embed *.tmpl
var TemplatesFS embed.FS

func Asset(name string) ([]byte, error) {
	if name[0] == '/' {
		name = name[1:]
	}
	return fs.ReadFile(TemplatesFS, name)
}

func AssetNames() []string {
	var results []string
	_ = fs.WalkDir(TemplatesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if d.Type().IsRegular() {
			results = append(results, path)
		}
		return nil
	})
	return results
}

func AssetDir(dirName string) ([]string, error) {
	files, err := fs.ReadDir(TemplatesFS, dirName)
	if err != nil {
		return nil, err
	}
	results := make([]string, 0, len(files))
	for _, file := range files {
		results = append(results, file.Name())
	}
	return results, nil
}
