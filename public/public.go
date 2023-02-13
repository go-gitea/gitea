// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic

package public

import (
	"embed"
	"io/fs"
)

// PublicFS contains the public assets filesystem.
//
//go:embed css fonts img js vendor *.js
var PublicFS embed.FS

func Asset(name string) ([]byte, error) {
	return fs.ReadFile(PublicFS, name)
}

func AssetNames() []string {
	var results []string
	_ = fs.WalkDir(PublicFS, ".", func(path string, d fs.DirEntry, err error) error {
		if d.Type().IsRegular() {
			results = append(results, path)
		}
		return nil
	})
	return results
}
