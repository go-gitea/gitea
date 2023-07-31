// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"html/template"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/util"
)

type materialIconsData struct {
	IconDefinitions map[string]*struct {
		IconPath    string `json:"iconPath"`
		IconContent string `json:"-"`
	} `json:"iconDefinitions"`
	FileNames      map[string]string `json:"fileNames"`
	FolderNames    map[string]string `json:"folderNames"`
	FileExtensions map[string]string `json:"fileExtensions"`
	LanguageIds    map[string]string `json:"languageIds"`
}

type MaterialIconProvider struct {
	mu sync.RWMutex

	fs             http.FileSystem
	packFile       string
	packFileTime   time.Time
	lastStatTime   time.Time
	reloadInterval time.Duration

	materialIcons *materialIconsData
}

var (
	materialIconProvider     *MaterialIconProvider
	materialIconProviderOnce sync.Once
)

func DefaultMaterialIconProvider() *MaterialIconProvider {
	materialIconProviderOnce.Do(func() {
		materialIconProvider = NewMaterialIconProvider(options.AssetFS(), "fileicon/material.tgz")
	})
	return materialIconProvider
}

func NewMaterialIconProvider(fs http.FileSystem, packFile string) *MaterialIconProvider {
	return &MaterialIconProvider{fs: fs, packFile: packFile, reloadInterval: time.Second}
}

func (m *MaterialIconProvider) loadDataFromPack(pack http.File) (*materialIconsData, error) {
	gzf, err := gzip.NewReader(pack)
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{}
	tarReader := tar.NewReader(gzf)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		files[util.PathJoinRelX(header.Name)], err = io.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}
	}

	iconsData := materialIconsData{}
	err = json.Unmarshal(files["package/dist/material-icons.json"], &iconsData)
	if err != nil {
		return nil, err
	}

	for name, icon := range iconsData.IconDefinitions {
		iconContent := files[path.Join("package/dist", icon.IconPath)]
		iconsData.IconDefinitions[name].IconContent = string(svg.Normalize(iconContent, 16))
	}

	return &iconsData, nil
}

func (m *MaterialIconProvider) loadData() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.lastStatTime) > m.reloadInterval {
		m.lastStatTime = time.Now()

		f, err := m.fs.Open(m.packFile)
		if err != nil {
			log.Error("Failed to open material icon pack file: %v", err)
			return
		}
		defer f.Close()

		fileInfo, err := f.Stat()
		if err != nil {
			log.Error("Failed to stat material icon pack file: %v", err)
			return
		}
		if fileInfo.ModTime().Equal(m.packFileTime) {
			return
		}

		iconsData, err := m.loadDataFromPack(f)
		if err != nil {
			log.Error("Failed to load material icon pack file: %v", err)
			return
		}
		m.materialIcons = iconsData
		m.packFileTime = fileInfo.ModTime()
	}
}

func (m *MaterialIconProvider) FileIcon(ctx context.Context, entry *git.TreeEntry) template.HTML {
	m.mu.RLock()
	if time.Since(m.lastStatTime) > m.reloadInterval {
		m.mu.RUnlock()
		m.loadData()
		m.mu.RLock()
	}
	defer m.mu.RUnlock()

	if m.materialIcons == nil {
		return fileIconBasic(ctx, entry)
	}

	if entry.IsLink() {
		if te, err := entry.FollowLink(); err == nil && te.IsDir() {
			return svg.RenderHTML("material-folder-symlink")
		}
		return svg.RenderHTML("octicon-file-symlink-file") // TODO: find some better icons for them
	}

	name := m.findIconName(entry)
	if name == "folder" {
		// the material icon pack's "folder" icon doesn't look good, so use our built-in one
		return svg.RenderHTML("material-folder-generic")
	}
	if iconDef, ok := m.materialIcons.IconDefinitions[name]; ok && iconDef.IconContent != "" {
		return template.HTML(iconDef.IconContent)
	}
	return svg.RenderHTML("octicon-file")
}

func (m *MaterialIconProvider) findIconName(entry *git.TreeEntry) string {
	if entry.IsSubModule() {
		return "folder-git"
	}

	iconsData := m.materialIcons
	fileName := path.Base(entry.Name())

	if entry.IsDir() {
		if s, ok := iconsData.FolderNames[fileName]; ok {
			return s
		}
		if s, ok := iconsData.FolderNames[strings.ToLower(fileName)]; ok {
			return s
		}
		return "folder"
	}

	if s, ok := iconsData.FileNames[fileName]; ok {
		return s
	}
	if s, ok := iconsData.FileNames[strings.ToLower(fileName)]; ok {
		return s
	}

	for i := len(fileName) - 1; i >= 0; i-- {
		if fileName[i] == '.' {
			ext := fileName[i+1:]
			if s, ok := iconsData.FileExtensions[ext]; ok {
				return s
			}
		}
	}

	return "file"
}
