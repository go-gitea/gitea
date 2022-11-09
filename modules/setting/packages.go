// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"math"
	"net/url"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"

	"github.com/dustin/go-humanize"
	ini "gopkg.in/ini.v1"
)

// Package registry settings
var (
	Packages = struct {
		Storage
		Enabled           bool
		ChunkedUploadPath string
		RegistryHost      string

		LimitTotalOwnerCount int64
		LimitTotalOwnerSize  int64
		LimitSizeComposer    int64
		LimitSizeConan       int64
		LimitSizeContainer   int64
		LimitSizeGeneric     int64
		LimitSizeHelm        int64
		LimitSizeMaven       int64
		LimitSizeNpm         int64
		LimitSizeNuGet       int64
		LimitSizePub         int64
		LimitSizePyPI        int64
		LimitSizeRubyGems    int64
		LimitSizeVagrant     int64
	}{
		Enabled:              true,
		LimitTotalOwnerCount: -1,
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

	Packages.LimitTotalOwnerSize = mustBytes(sec, "LIMIT_TOTAL_OWNER_SIZE")
	Packages.LimitSizeComposer = mustBytes(sec, "LIMIT_SIZE_COMPOSER")
	Packages.LimitSizeConan = mustBytes(sec, "LIMIT_SIZE_CONAN")
	Packages.LimitSizeContainer = mustBytes(sec, "LIMIT_SIZE_CONTAINER")
	Packages.LimitSizeGeneric = mustBytes(sec, "LIMIT_SIZE_GENERIC")
	Packages.LimitSizeHelm = mustBytes(sec, "LIMIT_SIZE_HELM")
	Packages.LimitSizeMaven = mustBytes(sec, "LIMIT_SIZE_MAVEN")
	Packages.LimitSizeNpm = mustBytes(sec, "LIMIT_SIZE_NPM")
	Packages.LimitSizeNuGet = mustBytes(sec, "LIMIT_SIZE_NUGET")
	Packages.LimitSizePub = mustBytes(sec, "LIMIT_SIZE_PUB")
	Packages.LimitSizePyPI = mustBytes(sec, "LIMIT_SIZE_PYPI")
	Packages.LimitSizeRubyGems = mustBytes(sec, "LIMIT_SIZE_RUBYGEMS")
	Packages.LimitSizeVagrant = mustBytes(sec, "LIMIT_SIZE_VAGRANT")
}

func mustBytes(section *ini.Section, key string) int64 {
	const noLimit = "-1"

	value := section.Key(key).MustString(noLimit)
	if value == noLimit {
		return -1
	}
	bytes, err := humanize.ParseBytes(value)
	if err != nil || bytes > math.MaxInt64 {
		return -1
	}
	return int64(bytes)
}
