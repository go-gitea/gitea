// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon_test

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/fileicon"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{FixtureFiles: []string{}})
}

func TestFindIconName(t *testing.T) {
	unittest.PrepareTestEnv(t)
	p := fileicon.DefaultMaterialIconProvider()
	assert.Equal(t, "php", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.php", EntryMode: git.EntryModeBlob}))
	assert.Equal(t, "php", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.PHP", EntryMode: git.EntryModeBlob}))
	assert.Equal(t, "javascript", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.js", EntryMode: git.EntryModeBlob}))
	assert.Equal(t, "visualstudio", p.FindIconName(&fileicon.EntryInfo{BaseName: "foo.vba", EntryMode: git.EntryModeBlob}))
}
