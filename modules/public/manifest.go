// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !bindata
// +build !bindata

package public

import (
	"io"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var (
	manifest         map[string]string
	manifestName     = "assets-manifest.json"
	manifestMutex    = &sync.Mutex{}
	manifestModified time.Time
)

func readManifestFile(fs http.FileSystem) (http.File, fs.FileInfo, error) {
	f, err := fs.Open(manifestName)
	if err != nil {
		return nil, nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}

	return f, fi, nil
}

func readManifest(fs http.FileSystem) map[string]string {
	manifestMutex.Lock()
	var assetMap map[string]string

	f, fi, err := readManifestFile(fs)
	if err != nil {
		log.Error("[Static] Failed to open %q: %v", manifestName, err)
		return assetMap
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		log.Error("[Static] Failed to read %q: %v", manifestName, err)
		return assetMap
	}

	err = json.Unmarshal(bytes, &assetMap)
	if err != nil {
		log.Error("[Static] Failed to parse %q: %v", manifestName, err)
		return assetMap
	}

	manifestModified = fi.ModTime()
	manifestMutex.Unlock()
	return assetMap
}

// ResolveWithManifest turns /js/index.js into /js/index.5ed90373e37c.js using assets-manifest.json
func ResolveWithManifest(fs http.FileSystem, file string) string {
	if len(manifest) == 0 {
		manifest = readManifest(fs)
	}

	// in development, the manifest can frequently change, check and reload if necessary
	if !setting.IsProd {
		f, fi, err := readManifestFile(fs)
		if err != nil {
			log.Error("[Static] Failed to open %q: %v", manifestName, err)
		} else {
			defer f.Close()
		}

		if fi.ModTime().After(manifestModified) {
			manifest = readManifest(fs)
		}
	}

	if mappedFile, ok := manifest[file]; ok {
		return mappedFile
	}
	return file
}
