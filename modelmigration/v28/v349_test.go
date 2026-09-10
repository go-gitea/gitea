// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"bytes"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandActionScheduleContent(t *testing.T) {
	if !setting.Database.Type.IsMySQL() {
		t.Skip("Only MySQL limits BLOB columns to 65,535 bytes")
	}

	type ActionSchedule struct {
		ID      int64  `xorm:"pk autoincr"`
		Content []byte `xorm:"BLOB"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionSchedule))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, ExpandActionScheduleContent(t.Context(), x))

	tables := migrationtest.LoadTableSchemasMap(t, x)
	assert.Equal(t, "LONGBLOB", tables["action_schedule"].GetColumn("content").SQLType.Name)

	content := bytes.Repeat([]byte("x"), 65_536)
	_, err := x.Insert(&ActionSchedule{Content: content})
	require.NoError(t, err)

	var stored ActionSchedule
	has, err := x.Get(&stored)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, content, stored.Content)
}
