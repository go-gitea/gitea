// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_MigrateIniToDatabase(t *testing.T) {
	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Setting))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	assert.NoError(t, MigrateIniToDatabase(x))

	cnt, err := x.Table("system_setting").Where("setting_key LIKE 'ui.%'").Count()
	assert.NoError(t, err)
	assert.EqualValues(t, 13, cnt)
}
