// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon

import (
	"html/template"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/svg"
)

type FileEntry struct {
	Name            string
	EntryMode       git.EntryMode
	FollowEntryMode git.EntryMode
}

func GetFileEntryByTreeEntry(entry *git.TreeEntry) *FileEntry {
	if entry.IsLink() {
		if te, err := entry.FollowLink(); err == nil && te.IsDir() {
			return &FileEntry{
				Name:            entry.Name(),
				EntryMode:       entry.Mode(),
				FollowEntryMode: te.Mode(),
			}
		}
	}
	return &FileEntry{
		Name:      entry.Name(),
		EntryMode: entry.Mode(),
	}
}

func BasicThemeFolderIconName(isOpen bool) string {
	if isOpen {
		return "octicon-file-directory-open-fill"
	}
	return "octicon-file-directory-fill"
}

func BasicThemeFolderIconWithOpenStatus(isOpen bool) template.HTML {
	return svg.RenderHTML(BasicThemeFolderIconName(isOpen))
}

func BasicThemeIconWithOpenStatus(entry *FileEntry, isOpen bool) template.HTML {
	// TODO: add "open icon" support
	svgName := "octicon-file"
	switch {
	case entry.EntryMode.IsLink():
		svgName = "octicon-file-symlink-file"
		if entry.FollowEntryMode.IsDir() {
			svgName = "octicon-file-directory-symlink"
		}
	case entry.EntryMode.IsDir():
		svgName = BasicThemeFolderIconName(isOpen)
	case entry.EntryMode.IsSubModule():
		svgName = "octicon-file-submodule"
	}
	return svg.RenderHTML(svgName)
}
