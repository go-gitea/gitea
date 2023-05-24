// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

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

	assert.Len(t, GitConfig.Options, 2)
	assert.EqualValues(t, "1", GitConfig.Options["a.b"])
	assert.EqualValues(t, "histogram", GitConfig.Options["diff.algorithm"])

	cfg, err = NewConfigProviderFromData(`
[git.config]
diff.algorithm = other
`)
	assert.NoError(t, err)
	loadGitFrom(cfg)

	assert.Len(t, GitConfig.Options, 1)
	assert.EqualValues(t, "other", GitConfig.Options["diff.algorithm"])
}
