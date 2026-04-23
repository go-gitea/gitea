// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"io"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type manifestEntry struct {
	File    string   `json:"file"`
	Name    string   `json:"name"`
	IsEntry bool     `json:"isEntry"`
	CSS     []string `json:"css"`
}

type manifestDataStruct struct {
	paths     map[string]string // unhashed path -> hashed path
	names     map[string]string // hashed path -> entry name
	modTime   int64
	checkTime time.Time
}

var (
	manifestData atomic.Pointer[manifestDataStruct]
	manifestFS   = sync.OnceValue(AssetFS)
)

const manifestPath = "assets/.vite/manifest.json"

func parseManifest(data []byte) (map[string]string, map[string]string) {
	var manifest map[string]manifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		log.Error("Failed to parse frontend manifest: %v", err)
		return nil, nil
	}

	paths := make(map[string]string)
	names := make(map[string]string)
	for _, entry := range manifest {
		if !entry.IsEntry || entry.Name == "" {
			continue
		}
		// Build unhashed key from file path: "js/index.js", "css/theme-gitea-dark.css"
		dir := path.Dir(entry.File)
		ext := path.Ext(entry.File)
		key := dir + "/" + entry.Name + ext
		paths[key] = entry.File
		names[entry.File] = entry.Name
		// Map associated CSS files, e.g. "css/index.css" -> "css/index.B3zrQPqD.css"
		// FIXME: INCORRECT-VITE-MANIFEST-PARSER: the logic is wrong, Vite manifest doesn't work this way
		// It just happens to be correct for the current modules dependencies
		for _, css := range entry.CSS {
			cssKey := path.Dir(css) + "/" + entry.Name + path.Ext(css)
			paths[cssKey] = css
			names[css] = entry.Name
		}
	}
	return paths, names
}

func reloadManifest(existingData *manifestDataStruct) *manifestDataStruct {
	now := time.Now()
	data := existingData
	if data != nil && now.Sub(data.checkTime) < time.Second {
		// a single request triggers multiple calls to getHashedPath
		// do not check the manifest file too frequently
		return data
	}

	f, err := manifestFS().Open(manifestPath)
	if err != nil {
		log.Error("Failed to open frontend manifest: %v", err)
		return data
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Error("Failed to stat frontend manifest: %v", err)
		return data
	}

	needReload := data == nil || fi.ModTime().UnixNano() != data.modTime
	if !needReload {
		return data
	}
	manifestContent, err := io.ReadAll(f)
	if err != nil {
		log.Error("Failed to read frontend manifest: %v", err)
		return data
	}
	return storeManifestFromBytes(manifestContent, fi.ModTime().UnixNano(), now)
}

func storeManifestFromBytes(manifestContent []byte, modTime int64, checkTime time.Time) *manifestDataStruct {
	paths, names := parseManifest(manifestContent)
	data := &manifestDataStruct{
		paths:     paths,
		names:     names,
		modTime:   modTime,
		checkTime: checkTime,
	}
	manifestData.Store(data)
	return data
}

func getManifestData() *manifestDataStruct {
	data := manifestData.Load()

	// In production the manifest is immutable (embedded in the binary).
	// In dev mode, check if it changed on disk (for watch-frontend).
	if data == nil || !setting.IsProd {
		data = reloadManifest(data)
	}
	if data == nil {
		data = &manifestDataStruct{}
	}
	return data
}

// AssetURI returns the URI for a frontend asset.
// It may return a relative path or a full URL depending on the StaticURLPrefix setting.
// In Vite dev mode, known entry points are mapped to their source paths
// so the reverse proxy serves them from the Vite dev server.
// In production, it resolves the content-hashed path from the manifest.
func AssetURI(originPath string) string {
	if IsViteDevMode() {
		if src := viteDevSourceURL(originPath); src != "" {
			return src
		}
		// it should be caused by incorrect vite config
		setting.PanicInDevOrTesting("Failed to locate local path for managed asset URI: %s", originPath)
	}

	// Try to resolve an unhashed asset path (origin path) to its content-hashed path from the frontend manifest.
	// Example: "js/index.js" -> "js/index.C6Z2MRVQ.js"
	data := getManifestData()
	assetPath := data.paths[originPath]
	if assetPath == "" {
		// it should be caused by either: "incorrect vite config" or "user's custom theme"
		assetPath = originPath
		if !setting.IsProd {
			log.Warn("Failed to find managed asset URI for origin path: %s", originPath)
		}
	}

	return setting.StaticURLPrefix + "/assets/" + assetPath
}

// AssetNameFromHashedPath returns the asset entry name for a given hashed asset path.
// Example: returns "theme-gitea-dark" for "css/theme-gitea-dark.CyAaQnn5.css".
// Returns empty string if the path is not found in the manifest.
func AssetNameFromHashedPath(hashedPath string) string {
	return getManifestData().names[hashedPath]
}
