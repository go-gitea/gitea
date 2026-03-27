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
	paths     map[string]string
	modTime   int64
	checkTime time.Time
}

var (
	manifestData atomic.Pointer[manifestDataStruct]
	manifestFS   = sync.OnceValue(AssetFS)
)

const manifestPath = "assets/.vite/manifest.json"

func parseManifest(data []byte) map[string]string {
	var manifest map[string]manifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		log.Error("Failed to parse frontend manifest: %v", err)
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

func reloadManifest() *manifestDataStruct {
	now := time.Now()
	data := manifestData.Load()
	if data != nil && now.Sub(data.checkTime) < time.Second {
		// a single request triggers multiple calls to getAssetPath
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
	data = &manifestDataStruct{
		paths:     parseManifest(manifestContent),
		modTime:   fi.ModTime().UnixNano(),
		checkTime: now,
	}
	manifestData.Store(data)
	return data
}

func getManifestPaths() map[string]string {
	data := manifestData.Load()

	// In production the manifest is immutable (embedded in the binary).
	// In dev mode, check if it changed on disk (for watch-frontend).
	if data == nil || !setting.IsProd {
		data = reloadManifest()
	}
	if data != nil {
		return data.paths
	}
	return nil
}

// getAssetPath resolves an unhashed asset path to its content-hashed path from the frontend manifest.
// Example: getAssetPath("js/index.js") returns "js/index.C6Z2MRVQ.js"
// Falls back to returning the input path unchanged if the manifest is unavailable.
func getAssetPath(name string) string {
	paths := getManifestPaths()
	if p, ok := paths[name]; ok {
		return p
	}
	return name
}
