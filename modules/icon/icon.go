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
	"code.gitea.io/gitea/modules/log"
)

type Map struct {
	FileNames      map[string]string `json:"fileNames"`
	FolderNames    map[string]string `json:"folderNames"`
	FileExtensions map[string]string `json:"fileExtensions"`
	LanguageIds    map[string]string `json:"languageIds"`
}

var iconMap = Map{
	FileNames:      make(map[string]string),
	FolderNames:    make(map[string]string),
	FileExtensions: make(map[string]string),
	LanguageIds:    make(map[string]string),
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
	if entry.IsLink() {
		te, err := entry.FollowLink()
		if err != nil {
			log.Debug(err.Error())
			return "octicon-file-symlink-file"
		}
		if te.IsDir() {
			return "octicon-file-submodule"
		}
		return "octicon-file-symlink-file"
	}
	return "material-" + lookForMaterialMatch(entry)
}

func lookForMaterialMatch(entry *git.TreeEntry) string {
	if entry.IsSubModule() {
		return "folder-git"
	}

	fileName := entry.Name()

	if !entry.IsDir() {
		if iconMap.FileNames[fileName] != "" {
			return iconMap.FileNames[fileName]
		}
		
		lowerFileName := strings.ToLower(fileName)
		if iconMap.FileNames[lowerFileName] != "" {
			return iconMap.FileNames[lowerFileName]
		}
		
		fileExtension := strings.TrimPrefix(path.Ext(fileName), ".")
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
	
	lowerFileName := strings.ToLower(fileName)
	if iconMap.FolderNames[lowerFileName] != "" {
		return iconMap.FolderNames[lowerFileName]
	}
	return "folder"
}
