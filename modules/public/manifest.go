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

// https://vite.dev/guide/backend-integration
type manifestEntry struct {
	File string   `json:"file"`
	Name string   `json:"name"`
	CSS  []string `json:"css"`
}

type manifestDataStruct struct {
	entries   map[string]*manifestEntry // source path -> entry
	names     map[string]string         // content-hashed output file -> entry name
	modTime   int64
	checkTime time.Time
}

var (
	manifestData atomic.Pointer[manifestDataStruct]
	manifestFS   = sync.OnceValue(AssetFS)
)

const manifestPath = "assets/.vite/manifest.json"

func parseManifest(data []byte) (entries map[string]*manifestEntry, names map[string]string) {
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Error("Failed to parse frontend manifest: %v", err)
		return nil, nil
	}
	names = make(map[string]string, len(entries))
	for _, entry := range entries {
		names[entry.File] = entry.Name
	}
	return entries, names
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
	entries, names := parseManifest(manifestContent)
	data := &manifestDataStruct{
		entries:   entries,
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

// AssetURI resolves a frontend asset by its source path (the Vite manifest key, e.g.
// "web_src/js/index.ts"). Dev mode serves the source file; production resolves the hashed output.
func AssetURI(srcPath string) string {
	if IsViteDevMode() {
		if src := viteDevSourceURL(srcPath); src != "" {
			return src
		}
		setting.PanicInDevOrTesting("Failed to locate source file for asset: %s", srcPath)
	}

	if entry := getManifestData().entries[srcPath]; entry != nil {
		return setting.StaticURLPrefix + "/assets/" + entry.File
	}
	// The only expected manifest miss is a user's custom theme CSS, served as a static asset
	// under "/assets/css/". Anything else is a misconfigured or missing entry.
	if path.Ext(srcPath) == ".css" {
		return setting.StaticURLPrefix + "/assets/css/" + path.Base(srcPath)
	}
	log.Error("asset not found in frontend manifest: %s", srcPath)
	return setting.StaticURLPrefix + "/assets/" + path.Base(srcPath)
}

// AssetCSSURI returns the stylesheet URL for a JS entry. Dev serves devStylesheetSrc (the source
// file); prod returns the entry's CSS from the manifest.
func AssetCSSURI(jsEntrySrc, devStylesheetSrc string) string {
	if IsViteDevMode() {
		return viteDevSourceURL(devStylesheetSrc)
	}
	if entry := getManifestData().entries[jsEntrySrc]; entry != nil && len(entry.CSS) > 0 {
		return setting.StaticURLPrefix + "/assets/" + entry.CSS[0]
	}
	return ""
}

// AssetNameFromHashedPath returns the asset entry name for a given hashed asset path.
// Example: returns "theme-gitea-dark" for "css/theme-gitea-dark.CyAaQnn5.css".
// Returns empty string if the path is not found in the manifest.
func AssetNameFromHashedPath(hashedPath string) string {
	return getManifestData().names[hashedPath]
}
