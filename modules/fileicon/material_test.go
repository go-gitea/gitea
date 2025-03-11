// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fileicon_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/fileicon"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{FixtureFiles: []string{}})
}

func TestFindIconName(t *testing.T) {
	unittest.PrepareTestEnv(t)
	p := fileicon.DefaultMaterialIconProvider()
	assert.Equal(t, "php", p.FindIconName("foo.php", false))
	assert.Equal(t, "php", p.FindIconName("foo.PHP", false))
	assert.Equal(t, "javascript", p.FindIconName("foo.js", false))
	assert.Equal(t, "visualstudio", p.FindIconName("foo.vba", false))
}
