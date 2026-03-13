// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"io"
	"path"
	"sync"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

type viteManifestEntry struct {
	File    string   `json:"file"`
	Name    string   `json:"name"`
	IsEntry bool     `json:"isEntry"`
	CSS     []string `json:"css"`
}

var (
	manifestMu      sync.RWMutex
	manifestPaths   map[string]string
	manifestModTime int64
)

func parseManifest(data []byte) map[string]string {
	var manifest map[string]viteManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		log.Error("Failed to parse Vite manifest: %v", err)
		return nil
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
		// Map associated CSS files, e.g. "css/index.css" -> "css/index.B3zrQPqD.css"
		for _, css := range entry.CSS {
			cssKey := path.Dir(css) + "/" + entry.Name + path.Ext(css)
			paths[cssKey] = css
		}
	}
	return paths
}

func getManifestPaths() map[string]string {
	manifestMu.RLock()
	if manifestPaths != nil {
		f, err := AssetFS().Open("assets/.vite/manifest.json")
		if err != nil {
			manifestMu.RUnlock()
			return manifestPaths
		}
		fi, err := f.Stat()
		f.Close()
		if err != nil || fi.ModTime().UnixNano() == manifestModTime {
			manifestMu.RUnlock()
			return manifestPaths
		}
	}
	manifestMu.RUnlock()

	manifestMu.Lock()
	defer manifestMu.Unlock()

	// Double-check after acquiring write lock
	f, err := AssetFS().Open("assets/.vite/manifest.json")
	if err != nil {
		log.Error("Failed to open Vite manifest: %v", err)
		if manifestPaths == nil {
			manifestPaths = make(map[string]string)
		}
		return manifestPaths
	}
	fi, err := f.Stat()
	if err == nil && manifestPaths != nil && fi.ModTime().UnixNano() == manifestModTime {
		f.Close()
		return manifestPaths
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		log.Error("Failed to read Vite manifest: %v", err)
		if manifestPaths == nil {
			manifestPaths = make(map[string]string)
		}
		return manifestPaths
	}

	paths := parseManifest(data)
	if paths == nil {
		paths = make(map[string]string)
	}
	manifestPaths = paths
	if fi != nil {
		manifestModTime = fi.ModTime().UnixNano()
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
