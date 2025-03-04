// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_MigrateIniToDatabase(t *testing.T) {
	if err := db.InitEngine(context.Background()); err != nil {
		t.Fatal(err)
	}
	x := unittest.GetXORMEngine()

	assert.NoError(t, MigrateIniToDatabase(x))

	cnt, err := x.Table("system_setting").Where("setting_key LIKE 'ui.%'").Count()
	assert.NoError(t, err)
	assert.EqualValues(t, 21, cnt)
}
