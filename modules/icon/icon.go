// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This file was ported from https://github.com/Claudiohbsantos/github-material-icons-extension/blob/ff97e50980/src/lib/replace-icon.js
// to go with small changes.

package icon

import (
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
)

type Map struct {
	FileNames      map[string]string `json:"fileNames"`
	FolderNames    map[string]string `json:"folderNames"`
	FileExtensions map[string]string `json:"fileExtensions"`
	LanguageIds    map[string]string `json:"languageIds"`
	Light          Light             `json:"light"`
}

type Light struct {
	FileNames      map[string]string `json:"fileNames"`
	FolderNames    map[string]string `json:"folderNames"`
	FileExtensions map[string]string `json:"fileExtensions"`
}

var iconMap = Map{
	FileNames:      make(map[string]string),
	FolderNames:    make(map[string]string),
	FileExtensions: make(map[string]string),
	LanguageIds:    make(map[string]string),
	Light:          Light{},
}

func Init() error {
	data, err := os.ReadFile("assets/material-icons.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &iconMap)
	if err != nil {
		return err
	}

	return nil
}

// EntryIcon returns the icon for the given git entry
func EntryIcon(entry *git.TreeEntry) string {
	return "material-" + lookForMaterialMatch(entry)
}

func lookForMaterialMatch(entry *git.TreeEntry) string {
	if entry.IsSubModule() {
		return "folder-git"
	}
	if entry.IsLink() {
		return "folder-symlink"
	}

	fileName := entry.Name()
	lowerFileName := strings.ToLower(fileName)
	fileExtension := strings.TrimPrefix(path.Ext(fileName), ".")

	if !entry.IsDir() {
		if iconMap.FileNames[fileName] != "" {
			return iconMap.FileNames[fileName]
		}
		if iconMap.FileNames[lowerFileName] != "" {
			return iconMap.FileNames[lowerFileName]
		}
		if iconMap.FileExtensions[fileExtension] != "" {
			return iconMap.FileExtensions[fileExtension]
		}
		if iconMap.LanguageIds[fileExtension] != "" {
			return iconMap.LanguageIds[fileExtension]
		}
		return "file"
	}

	if iconMap.FolderNames[fileName] != "" {
		return iconMap.FolderNames[fileName]
	}
	if iconMap.FolderNames[lowerFileName] != "" {
		return iconMap.FolderNames[lowerFileName]
	}
	return "folder"
}
