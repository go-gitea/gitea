// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"os"
	"path"
	"path/filepath"
	"sync"

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
	manifestMu      sync.RWMutex
	manifestPaths   map[string]string
	manifestModTime int64
)

func manifestDiskPath() string {
	return filepath.Join(setting.StaticRootPath, "public", "assets", ".vite", "manifest.json")
}

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
	diskPath := manifestDiskPath()

	manifestMu.RLock()
	if manifestPaths != nil {
		fi, statErr := os.Stat(diskPath)
		if statErr != nil || fi.ModTime().UnixNano() == manifestModTime {
			paths := manifestPaths
			manifestMu.RUnlock()
			return paths
		}
	}
	manifestMu.RUnlock()

	manifestMu.Lock()
	defer manifestMu.Unlock()

	// Double-check after acquiring write lock
	fi, statErr := os.Stat(diskPath)
	if manifestPaths != nil {
		if statErr != nil || fi.ModTime().UnixNano() == manifestModTime {
			return manifestPaths
		}
	}

	// Read from disk if available, otherwise from AssetFS (bindata)
	var data []byte
	var err error
	if statErr == nil {
		data, err = os.ReadFile(diskPath)
	} else {
		data, err = AssetFS().ReadFile("assets", ".vite", "manifest.json")
	}
	if err != nil {
		log.Error("Failed to read Vite manifest: %v", err)
		manifestPaths = make(map[string]string)
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
