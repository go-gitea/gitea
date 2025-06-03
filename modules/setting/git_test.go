// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "1", GitConfig.Options["a.b"])
	assert.Equal(t, "histogram", GitConfig.Options["diff.algorithm"])

	cfg, err = NewConfigProviderFromData(`
[git.config]
diff.algorithm = other
`)
	assert.NoError(t, err)
	loadGitFrom(cfg)
	assert.Equal(t, "other", GitConfig.Options["diff.algorithm"])
}

func TestGitReflog(t *testing.T) {
	defer test.MockVariableValue(&Git)
	defer test.MockVariableValue(&GitConfig)

	// default reflog config without legacy options
	cfg, err := NewConfigProviderFromData(``)
	assert.NoError(t, err)
	loadGitFrom(cfg)

	assert.Equal(t, "true", GitConfig.GetOption("core.logAllRefUpdates"))
	assert.Equal(t, "90", GitConfig.GetOption("gc.reflogExpire"))

	// custom reflog config by legacy options
	cfg, err = NewConfigProviderFromData(`
[git.reflog]
ENABLED = false
EXPIRATION = 123
`)
	assert.NoError(t, err)
	loadGitFrom(cfg)

	assert.Equal(t, "false", GitConfig.GetOption("core.logAllRefUpdates"))
	assert.Equal(t, "123", GitConfig.GetOption("gc.reflogExpire"))
}
