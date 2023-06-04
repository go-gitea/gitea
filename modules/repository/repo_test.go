// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_cleanUpMigrateGitConfig(t *testing.T) {
	f, err := os.CreateTemp(os.TempDir(), "cleanUpMigrateGitConfig")
	assert.NoError(t, err)

	_, err = f.Write([]byte(`[core]
	repositoryformatversion = 0
	filemode = true
	bare = true
	ignorecase = true
	precomposeunicode = true
[remote "origin"]
	url = https://oauth2:xxxxxxxxxxxxx@github.com/lunny/tango.git
`))
	assert.NoError(t, err)

	p := f.Name()
	f.Close()

	assert.NoError(t, cleanUpMigrateGitConfig(p))

	bs, err := os.ReadFile(p)
	assert.NoError(t, err)
	assert.EqualValues(t, `[core]
	repositoryformatversion = 0
	filemode = true
	bare = true
	ignorecase = true
	precomposeunicode = true
`, string(bs))
}
