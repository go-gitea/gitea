// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitConfig(t *testing.T) {
	oldGit := Git
	oldGitConfig := GitConfig
	defer func() {
		Git = oldGit
		GitConfig = oldGitConfig
	}()

	cfg, err := NewConfigProviderFromData(`
[git.config]
a.b = 1
`)
	assert.NoError(t, err)
	loadGitFrom(cfg)
	assert.EqualValues(t, "1", GitConfig.Options["a.b"])
	assert.EqualValues(t, "histogram", GitConfig.Options["diff.algorithm"])

	cfg, err = NewConfigProviderFromData(`
[git.config]
diff.algorithm = other
`)
	assert.NoError(t, err)
	loadGitFrom(cfg)
	assert.EqualValues(t, "other", GitConfig.Options["diff.algorithm"])
}

func TestGitReflog(t *testing.T) {
	oldGit := Git
	oldGitConfig := GitConfig
	defer func() {
		Git = oldGit
		GitConfig = oldGitConfig
	}()

	// default reflog config without legacy options
	cfg, err := NewConfigProviderFromData(``)
	assert.NoError(t, err)
	loadGitFrom(cfg)

	assert.EqualValues(t, "true", GitConfig.GetOption("core.logAllRefUpdates"))
	assert.EqualValues(t, "90", GitConfig.GetOption("gc.reflogExpire"))
}

func TestLegacyRefLog(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[git.reflog]
ENABLED = false
EXPIRATION = 123
`)
	require.NoError(t, err)

	require.Error(t, checkForRemovedSettings(cfg))
}
