// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddWorkflowCallOriginalEventSupportToActionRunner(t *testing.T) {
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

	require.NoError(t, AddWorkflowCallOriginalEventSupportToActionRunner(t.Context(), x))

	var hasWorkflowCallOriginalEventSupport bool
	has, err := x.SQL(
		"SELECT has_workflow_call_original_event_support FROM action_runner WHERE id = ?",
		1,
	).Get(&hasWorkflowCallOriginalEventSupport)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, hasWorkflowCallOriginalEventSupport)
}
