// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
)

// Package registry settings
var (
	Packages = struct {
		Storage           *Storage
		Enabled           bool
		ChunkedUploadPath string
		RegistryHost      string

		LimitTotalOwnerCount int64
		LimitTotalOwnerSize  int64
		LimitSizeAlpine      int64
		LimitSizeCargo       int64
		LimitSizeChef        int64
		LimitSizeComposer    int64
		LimitSizeConan       int64
		LimitSizeConda       int64
		LimitSizeContainer   int64
		LimitSizeCran        int64
		LimitSizeDebian      int64
		LimitSizeGeneric     int64
		LimitSizeGo          int64
		LimitSizeHelm        int64
		LimitSizeMaven       int64
		LimitSizeNpm         int64
		LimitSizeNuGet       int64
		LimitSizePub         int64
		LimitSizePyPI        int64
		LimitSizeRpm         int64
		LimitSizeRubyGems    int64
		LimitSizeSwift       int64
		LimitSizeVagrant     int64
	}{
		Enabled:              true,
		LimitTotalOwnerCount: -1,
	}
)

func loadPackagesFrom(rootCfg ConfigProvider) (err error) {
	sec, _ := rootCfg.GetSection("packages")
	if sec == nil {
		Packages.Storage, err = getStorage(rootCfg, "packages", "", nil)
		return err
	}

	if err = sec.MapTo(&Packages); err != nil {
		return fmt.Errorf("failed to map Packages settings: %v", err)
	}

	Packages.Storage, err = getStorage(rootCfg, "packages", "", sec)
	if err != nil {
		return err
	}

	appURL, _ := url.Parse(AppURL)
	Packages.RegistryHost = appURL.Host

	Packages.ChunkedUploadPath = filepath.ToSlash(sec.Key("CHUNKED_UPLOAD_PATH").MustString("tmp/package-upload"))
	if !filepath.IsAbs(Packages.ChunkedUploadPath) {
		Packages.ChunkedUploadPath = filepath.ToSlash(filepath.Join(AppDataPath, Packages.ChunkedUploadPath))
	}

	if HasInstallLock(rootCfg) {
		if err := os.MkdirAll(Packages.ChunkedUploadPath, os.ModePerm); err != nil {
			return fmt.Errorf("unable to create chunked upload directory: %s (%v)", Packages.ChunkedUploadPath, err)
		}
	}

	Packages.LimitTotalOwnerSize = mustBytes(sec, "LIMIT_TOTAL_OWNER_SIZE")
	Packages.LimitSizeAlpine = mustBytes(sec, "LIMIT_SIZE_ALPINE")
	Packages.LimitSizeCargo = mustBytes(sec, "LIMIT_SIZE_CARGO")
	Packages.LimitSizeChef = mustBytes(sec, "LIMIT_SIZE_CHEF")
	Packages.LimitSizeComposer = mustBytes(sec, "LIMIT_SIZE_COMPOSER")
	Packages.LimitSizeConan = mustBytes(sec, "LIMIT_SIZE_CONAN")
	Packages.LimitSizeConda = mustBytes(sec, "LIMIT_SIZE_CONDA")
	Packages.LimitSizeContainer = mustBytes(sec, "LIMIT_SIZE_CONTAINER")
	Packages.LimitSizeCran = mustBytes(sec, "LIMIT_SIZE_CRAN")
	Packages.LimitSizeDebian = mustBytes(sec, "LIMIT_SIZE_DEBIAN")
	Packages.LimitSizeGeneric = mustBytes(sec, "LIMIT_SIZE_GENERIC")
	Packages.LimitSizeGo = mustBytes(sec, "LIMIT_SIZE_GO")
	Packages.LimitSizeHelm = mustBytes(sec, "LIMIT_SIZE_HELM")
	Packages.LimitSizeMaven = mustBytes(sec, "LIMIT_SIZE_MAVEN")
	Packages.LimitSizeNpm = mustBytes(sec, "LIMIT_SIZE_NPM")
	Packages.LimitSizeNuGet = mustBytes(sec, "LIMIT_SIZE_NUGET")
	Packages.LimitSizePub = mustBytes(sec, "LIMIT_SIZE_PUB")
	Packages.LimitSizePyPI = mustBytes(sec, "LIMIT_SIZE_PYPI")
	Packages.LimitSizeRpm = mustBytes(sec, "LIMIT_SIZE_RPM")
	Packages.LimitSizeRubyGems = mustBytes(sec, "LIMIT_SIZE_RUBYGEMS")
	Packages.LimitSizeSwift = mustBytes(sec, "LIMIT_SIZE_SWIFT")
	Packages.LimitSizeVagrant = mustBytes(sec, "LIMIT_SIZE_VAGRANT")
	return nil
}

func mustBytes(section ConfigSection, key string) int64 {
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
