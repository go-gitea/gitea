// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/require"
)

func Test_AddDisabledToActionRunner(t *testing.T) {
	type ActionRunner struct {
		ID   int64 `xorm:"pk autoincr"`
		Name string
	}

	x, deferable := base.PrepareTestEnv(t, 0, new(ActionRunner))
	defer deferable()

	_, err := x.Insert(&ActionRunner{Name: "runner"})
	require.NoError(t, err)

	require.NoError(t, AddDisabledToActionRunner(x))

	var isDisabled bool
	has, err := x.SQL("SELECT is_disabled FROM action_runner WHERE id = ?", 1).Get(&isDisabled)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, isDisabled)
}
