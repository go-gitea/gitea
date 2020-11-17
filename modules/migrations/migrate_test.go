// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMigrateWhiteBlocklist(t *testing.T) {
	setting.Migration.AllowlistedDomains = []string{"github.com"}
	assert.NoError(t, Init())

	allowed, err := isMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git")
	assert.False(t, allowed)
	assert.Error(t, err)

	allowed, err = isMigrateURLAllowed("https://github.com/go-gitea/gitea.git")
	assert.True(t, allowed)
	assert.NoError(t, err)

	setting.Migration.AllowlistedDomains = []string{}
	setting.Migration.BlocklistedDomains = []string{"github.com"}
	assert.NoError(t, Init())

	allowed, err = isMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git")
	assert.True(t, allowed)
	assert.NoError(t, err)

	allowed, err = isMigrateURLAllowed("https://github.com/go-gitea/gitea.git")
	assert.False(t, allowed)
	assert.Error(t, err)
}
