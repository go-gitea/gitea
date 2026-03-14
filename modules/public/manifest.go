// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"io"
	"path"
	"sync"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type viteManifestEntry struct {
	File    string   `json:"file"`
	Name    string   `json:"name"`
	IsEntry bool     `json:"isEntry"`
	CSS     []string `json:"css"`
}

var (
	manifestOnce    sync.Once
	manifestFS      *assetfs.LayeredFS
	manifestPaths   map[string]string
	manifestModTime int64
)

const manifestPath = "assets/.vite/manifest.json"

func parseManifest(data []byte) map[string]string {
	var manifest map[string]viteManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		log.Error("Failed to parse Vite manifest: %v", err)
		return make(map[string]string)
	}

	paths := make(map[string]string)
	for _, entry := range manifest {
		if !entry.IsEntry || entry.Name == "" {
			continue
		}
		// Build unhashed key from file path: "js/index.js", "css/theme-gitea-dark.css"
		dir := path.Dir(entry.File)
		ext := path.Ext(entry.File)
		key := dir + "/" + entry.Name + ext
		paths[key] = entry.File
		// Map associated CSS files, e.g. "css/index-domready.css" -> "css/index-domready.B3zrQPqD.css"
		for _, css := range entry.CSS {
			cssKey := path.Dir(css) + "/" + entry.Name + path.Ext(css)
			paths[cssKey] = css
		}
	}
	return paths
}

func initManifest() {
	manifestFS = AssetFS()
	reloadManifest()
}

func reloadManifest() {
	f, err := manifestFS.Open(manifestPath)
	if err != nil {
		log.Error("Failed to open Vite manifest: %v", err)
		manifestPaths = make(map[string]string)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err == nil {
		manifestModTime = fi.ModTime().UnixNano()
	}

	data, err := io.ReadAll(f)
	if err != nil {
		log.Error("Failed to read Vite manifest: %v", err)
		manifestPaths = make(map[string]string)
		return
	}

	manifestPaths = parseManifest(data)
}

func getManifestPaths() map[string]string {
	manifestOnce.Do(initManifest)

	// In production the manifest is immutable (embedded in the binary).
	// In dev mode, check if it changed on disk (for watch-frontend).
	if !setting.IsProd {
		f, err := manifestFS.Open(manifestPath)
		if err == nil {
			fi, err := f.Stat()
			f.Close()
			if err == nil && fi.ModTime().UnixNano() != manifestModTime {
				reloadManifest()
			}
		}
	}
	return manifestPaths
}

// GetAssetPath resolves an unhashed asset path to its content-hashed path from the Vite manifest.
// Example: GetAssetPath("js/index.js") returns "js/index.C6Z2MRVQ.js"
// Falls back to returning the input path unchanged if the manifest is unavailable.
func GetAssetPath(name string) string {
	paths := getManifestPaths()
	if p, ok := paths[name]; ok {
		return p
	}
	return name
}
