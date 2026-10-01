// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddSecretSaltToTwoFactor(t *testing.T) {
	type TwoFactor struct {
		ID     int64 `xorm:"pk autoincr"`
		UID    int64 `xorm:"UNIQUE"`
		Secret string
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(TwoFactor))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddSecretSaltToTwoFactor(t.Context(), x))

	tables := migrationtest.LoadTableSchemasMap(t, x)
	schema, ok := tables["two_factor"]
	require.True(t, ok)
	require.Contains(t, schema.ColumnsSeq(), "secret_salt")
}
