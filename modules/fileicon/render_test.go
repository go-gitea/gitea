// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon_test

import (
	"testing"

	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRenderEntryIconHTML_WithDifferentThemes(t *testing.T) {
	// Test that folder icons use the folder theme
	t.Run("FolderUsesBasicTheme", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.FileIconTheme, "material")()
		defer test.MockVariableValue(&setting.UI.FolderIconTheme, "basic")()

		folderEntry := &fileicon.EntryInfo{
			BaseName:  "testfolder",
			EntryMode: git.EntryModeTree,
		}

		html := fileicon.RenderEntryIconHTML(nil, folderEntry)
		// Basic theme renders octicon classes
		assert.Contains(t, string(html), "octicon-file-directory-fill")
	})

	t.Run("FileUsesMaterialTheme", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.FileIconTheme, "material")()
		defer test.MockVariableValue(&setting.UI.FolderIconTheme, "basic")()

		fileEntry := &fileicon.EntryInfo{
			BaseName:  "test.js",
			EntryMode: git.EntryModeBlob,
		}

		html := fileicon.RenderEntryIconHTML(nil, fileEntry)
		// Material theme for files renders material icons
		assert.Contains(t, string(html), "svg-mfi-")
	})

	t.Run("SymlinkToFolderUsesBasicTheme", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.FileIconTheme, "material")()
		defer test.MockVariableValue(&setting.UI.FolderIconTheme, "basic")()

		symlinkEntry := &fileicon.EntryInfo{
			BaseName:      "link",
			EntryMode:     git.EntryModeSymlink,
			SymlinkToMode: git.EntryModeTree,
		}

		html := fileicon.RenderEntryIconHTML(nil, symlinkEntry)
		// Symlinks to folders should use folder theme
		assert.Contains(t, string(html), "octicon-file-directory-symlink")
	})

	t.Run("BothMaterialTheme", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.FileIconTheme, "material")()
		defer test.MockVariableValue(&setting.UI.FolderIconTheme, "material")()

		folderEntry := &fileicon.EntryInfo{
			BaseName:  "testfolder",
			EntryMode: git.EntryModeTree,
		}

		html := fileicon.RenderEntryIconHTML(nil, folderEntry)
		// Material theme for folders renders material folder icons
		assert.Contains(t, string(html), "svg-mfi-")
	})
}
