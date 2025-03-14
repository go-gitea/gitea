// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigSubmodule(t *testing.T) {
	input := `
[core]
path = test

[submodule "submodule1"]
	path = path1
	url =	https://gitea.io/foo/foo
	#branch = b1

[other1]
branch = master

[submodule "submodule2"]
	path = path2
	url =	https://gitea.io/bar/bar
	branch = b2

[other2]
branch = main

[submodule "submodule3"]
	path = path3
	url =	https://gitea.io/xxx/xxx
`

	subModules, err := configParseSubModules(strings.NewReader(input))
	assert.NoError(t, err)
	assert.Len(t, subModules.cache, 3)

	sm1, _ := subModules.Get("path1")
	assert.Equal(t, &SubModule{Path: "path1", URL: "https://gitea.io/foo/foo", Branch: ""}, sm1)
	sm2, _ := subModules.Get("path2")
	assert.Equal(t, &SubModule{Path: "path2", URL: "https://gitea.io/bar/bar", Branch: "b2"}, sm2)
	sm3, _ := subModules.Get("path3")
	assert.Equal(t, &SubModule{Path: "path3", URL: "https://gitea.io/xxx/xxx", Branch: ""}, sm3)
}
