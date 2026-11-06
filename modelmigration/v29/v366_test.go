// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestDropUserOpenIDTable(t *testing.T) {
	type UserOpenID struct {
		ID int64 `xorm:"pk autoincr"`
	}

	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(UserOpenID))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}

	exists, err := x.IsTableExist("user_open_id")
	require.NoError(t, err)
	require.True(t, exists)
	require.NoError(t, DropUserOpenIDTable(t.Context(), x))
	exists, err = x.IsTableExist("user_open_id")
	require.NoError(t, err)
	require.False(t, exists)
	require.NoError(t, DropUserOpenIDTable(t.Context(), x))
}
