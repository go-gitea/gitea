// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/url"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
)

// Package registry settings
var (
	Packages = struct {
		Storage
		Enabled           bool
		ChunkedUploadPath string
		RegistryHost      string
	}{
		Enabled: true,
	}
)

func newPackages() {
	sec := Cfg.Section("packages")
	if err := sec.MapTo(&Packages); err != nil {
		log.Fatal("Failed to map Packages settings: %v", err)
	}

	Packages.Storage = getStorage("packages", "", nil)

	appURL, _ := url.Parse(AppURL)
	Packages.RegistryHost = appURL.Host

	Packages.ChunkedUploadPath = filepath.ToSlash(sec.Key("CHUNKED_UPLOAD_PATH").MustString("tmp/package-upload"))
	if !filepath.IsAbs(Packages.ChunkedUploadPath) {
		Packages.ChunkedUploadPath = filepath.ToSlash(filepath.Join(AppDataPath, Packages.ChunkedUploadPath))
	}

	if err := os.MkdirAll(Packages.ChunkedUploadPath, os.ModePerm); err != nil {
		log.Error("Unable to create chunked upload directory: %s (%v)", Packages.ChunkedUploadPath, err)
	}
}
