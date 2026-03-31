// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AddSecretSaltToTwoFactor(t *testing.T) {
	type TwoFactor struct {
		ID     int64 `xorm:"pk autoincr"`
		UID    int64 `xorm:"UNIQUE"`
		Secret string
	}

	x, deferrable := base.PrepareTestEnv(t, 0, new(TwoFactor))
	defer deferrable()

	require.NoError(t, AddSecretSaltToTwoFactor(x))

	table := base.LoadTableSchemasMap(t, x)["two_factor"]
	require.NotNil(t, table)
	assert.NotNil(t, table.GetColumn("secret_salt"))
	assert.NotNil(t, table.GetColumn("secret_algo"))
}
