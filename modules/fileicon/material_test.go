// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon_test

import (
	"os"
	"testing"

	"gitea.dev/modules/fileicon"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	setting.SetupGiteaTestEnv()
	os.Exit(m.Run())
}

func TestFindIconName(t *testing.T) {
	p := fileicon.DefaultMaterialIconProvider()
	assert.Equal(t, "php", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.php", EntryMode: git.EntryModeBlob}))
	assert.Equal(t, "php", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.PHP", EntryMode: git.EntryModeBlob}))
	assert.Equal(t, "javascript", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.js", EntryMode: git.EntryModeBlob}))
	assert.Equal(t, "visualstudio", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.vba", EntryMode: git.EntryModeBlob}))
}
