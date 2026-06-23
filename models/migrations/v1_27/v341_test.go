// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"gitea.dev/models/migrations/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestAddJobMaxParallel(t *testing.T) {
	type ActionRunJob struct {
		ID   int64  `xorm:"pk autoincr"`
		Name string `xorm:"VARCHAR(255)"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&ActionRunJob{Name: "job-a"})
	require.NoError(t, err)

	require.NoError(t, AddJobMaxParallel(x))

	var maxParallel int
	has, err := x.SQL("SELECT max_parallel FROM action_run_job WHERE id = ?", 1).Get(&maxParallel)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, 0, maxParallel)
}
