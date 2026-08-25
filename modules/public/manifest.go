// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"html"
	"html/template"
	"io"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

// https://vite.dev/guide/backend-integration
type manifestEntry struct {
	File    string   `json:"file"`
	Name    string   `json:"name"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
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

// devAssetURL returns a source file's Vite dev server URL, panicking in dev/testing if it's absent.
func devAssetURL(src string) string {
	if url := viteDevSourceURL(src); url != "" {
		return url
	}
	setting.PanicInDevOrTesting("Failed to locate source file for asset: %s", src)
	return ""
}

// AssetURI resolves a frontend asset by its source path (the Vite manifest key, e.g.
// "web_src/js/index.ts"). Dev mode serves the source file; production resolves the hashed output.
func AssetURI(srcPath string) string {
	if IsViteDevMode() {
		return devAssetURL(srcPath)
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

// AssetCSSLinks renders the <link> tags for a JS entry's stylesheets: the entry's CSS plus the CSS
// of every statically-imported chunk. Dev links devStylesheetSrc and lets the JS module inject the rest.
func AssetCSSLinks(jsEntrySrc, devStylesheetSrc string) template.HTML {
	var b strings.Builder
	for _, href := range entryStyleURLs(jsEntrySrc, devStylesheetSrc) {
		b.WriteString(`<link rel="stylesheet" href="` + html.EscapeString(href) + `">`)
	}
	return template.HTML(b.String())
}

func entryStyleURLs(jsEntrySrc, devStylesheetSrc string) []string {
	if IsViteDevMode() {
		return []string{devAssetURL(devStylesheetSrc)}
	}
	entries := getManifestData().entries
	var urls []string
	seen := make(map[string]bool)
	var walk func(key string)
	walk = func(key string) {
		entry := entries[key]
		if entry == nil || seen[key] {
			return
		}
		seen[key] = true
		for _, css := range entry.CSS {
			urls = append(urls, setting.StaticURLPrefix+"/assets/"+css)
		}
		for _, imp := range entry.Imports {
			walk(imp)
		}
	}
	walk(jsEntrySrc)
	return urls
}

// AssetNameFromHashedPath returns the asset entry name for a given hashed asset path.
// Example: returns "theme-gitea-dark" for "css/theme-gitea-dark.CyAaQnn5.css".
// Returns empty string if the path is not found in the manifest.
func AssetNameFromHashedPath(hashedPath string) string {
	return getManifestData().names[hashedPath]
}
