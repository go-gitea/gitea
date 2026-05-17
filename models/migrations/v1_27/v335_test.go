// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddMatrixFieldsToActionRunJob(t *testing.T) {
	type ActionRunJob struct {
		ID   int64 `xorm:"pk autoincr"`
		Name string
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&ActionRunJob{Name: "legacy"})
	require.NoError(t, err)

	require.NoError(t, AddMatrixFieldsToActionRunJob(x))

	// New columns must exist; existing rows default to empty / NULL.
	var rawMatrix string
	has, err := x.SQL("SELECT raw_matrix FROM action_run_job WHERE id = ?", 1).Get(&rawMatrix)
	require.NoError(t, err)
	require.True(t, has)
	require.Empty(t, rawMatrix)

	var matrixValues string
	has, err = x.SQL("SELECT COALESCE(matrix_values, '') FROM action_run_job WHERE id = ?", 1).Get(&matrixValues)
	require.NoError(t, err)
	require.True(t, has)
	require.Empty(t, matrixValues)
}
