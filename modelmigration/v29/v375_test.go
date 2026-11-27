// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPriorityToActionRunJob(t *testing.T) {
	type ActionRunJob struct {
		ID     int64 `xorm:"pk autoincr"`
		Status int   `xorm:"index"`
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunJob))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&ActionRunJob{Status: 5}, &ActionRunJob{Status: 1})
	require.NoError(t, err)
	require.NoError(t, AddPriorityToActionRunJob(t.Context(), x))

	// existing jobs become normal priority, so their queue order is unchanged
	var priorities []int
	require.NoError(t, x.Table("action_run_job").Cols("priority").Asc("id").Find(&priorities))
	assert.Equal(t, []int{0, 0}, priorities)
}
