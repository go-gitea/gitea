// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func gitConfigContains(sub string) bool {
	if b, err := os.ReadFile(HomeDir() + "/.gitconfig"); err == nil {
		return strings.Contains(string(b), sub)
	}
	return false
}

func TestGitConfig(t *testing.T) {
	assert.False(t, gitConfigContains("key-a"))

	assert.NoError(t, configSetNonExist("test.key-a", "val-a"))
	assert.True(t, gitConfigContains("key-a = val-a"))

	assert.NoError(t, configSetNonExist("test.key-a", "val-a-changed"))
	assert.False(t, gitConfigContains("key-a = val-a-changed"))

	assert.NoError(t, configSet("test.key-a", "val-a-changed"))
	assert.True(t, gitConfigContains("key-a = val-a-changed"))

	assert.NoError(t, configAddNonExist("test.key-b", "val-b"))
	assert.True(t, gitConfigContains("key-b = val-b"))

	assert.NoError(t, configAddNonExist("test.key-b", "val-2b"))
	assert.True(t, gitConfigContains("key-b = val-b"))
	assert.True(t, gitConfigContains("key-b = val-2b"))

	assert.NoError(t, configUnsetAll("test.key-b", "val-b"))
	assert.False(t, gitConfigContains("key-b = val-b"))
	assert.True(t, gitConfigContains("key-b = val-2b"))

	assert.NoError(t, configUnsetAll("test.key-b", "val-2b"))
	assert.False(t, gitConfigContains("key-b = val-2b"))

	assert.NoError(t, configSet("test.key-x", "*"))
	assert.True(t, gitConfigContains("key-x = *"))
	assert.NoError(t, configSetNonExist("test.key-x", "*"))
	assert.NoError(t, configUnsetAll("test.key-x", "*"))
	assert.False(t, gitConfigContains("key-x = *"))
}

func TestSyncConfig(t *testing.T) {
	oldGitConfig := setting.GitConfig
	defer func() {
		setting.GitConfig = oldGitConfig
	}()

	setting.GitConfig.Options["sync-test.cfg-key-a"] = "CfgValA"
	assert.NoError(t, syncGitConfig())
	assert.True(t, gitConfigContains("[sync-test]"))
	assert.True(t, gitConfigContains("cfg-key-a = CfgValA"))
}
