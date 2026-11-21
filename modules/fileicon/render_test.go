// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon_test

import (
	"testing"

	"gitea.dev/modules/fileicon"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRenderEntryIconHTML_WithDifferentThemes(t *testing.T) {
	// Test that folder icons use the folder theme
	for _, testCase := range []struct {
		name, folderTheme, expected string
		entry                       fileicon.EntryInfo
	}{
		// Basic theme renders octicon classes
		{
			"FolderUsesBasicTheme", "basic", "octicon-file-directory-fill",
			fileicon.EntryInfo{BaseName: "testfolder", EntryMode: git.EntryModeTree},
		},
		// Material theme for files renders material icons
		{
			"FileUsesMaterialTheme", "basic", "svg-mfi-",
			fileicon.EntryInfo{BaseName: "test.js", EntryMode: git.EntryModeBlob},
		},
		// Symlinks to folders should use folder theme
		{
			"SymlinkToFolderUsesBasicTheme", "basic", "octicon-file-directory-symlink",
			fileicon.EntryInfo{BaseName: "link", EntryMode: git.EntryModeSymlink, SymlinkToMode: git.EntryModeTree},
		},
		// Material theme for folders renders material folder icons
		{
			"BothMaterialTheme", "material", "svg-mfi-",
			fileicon.EntryInfo{BaseName: "testfolder", EntryMode: git.EntryModeTree},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer test.MockVariableValue(&setting.UI.FileIconTheme, "material")()
			defer test.MockVariableValue(&setting.UI.FolderIconTheme, testCase.folderTheme)()

			assert.Contains(t, string(fileicon.RenderEntryIconHTML(nil, &testCase.entry)), testCase.expected)
		})
	}
}
