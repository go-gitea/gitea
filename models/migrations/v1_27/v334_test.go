// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"gitea.dev/models/migrations/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddCancellingSupportToActionRunner(t *testing.T) {
	type ActionRunner struct {
		ID   int64 `xorm:"pk autoincr"`
		Name string
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunner))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&ActionRunner{Name: "runner"})
	require.NoError(t, err)

	require.NoError(t, AddCancellingSupportToActionRunner(x))

	var hasCancellingSupport bool
	has, err := x.SQL("SELECT has_cancelling_support FROM action_runner WHERE id = ?", 1).Get(&hasCancellingSupport)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, hasCancellingSupport)
}
