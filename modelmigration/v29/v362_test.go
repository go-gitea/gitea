// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPushMirrorConfigAndHistory(t *testing.T) {
	type PushMirror struct {
		ID         int64 `xorm:"pk autoincr"`
		RemoteName string
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(PushMirror))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}
	_, err := x.Insert(&PushMirror{RemoteName: "old"})
	require.NoError(t, err)

	require.NoError(t, AddPushMirrorConfigAndHistory(t.Context(), x))

	type PushMirrorAfter struct {
		RemoteName string
		Config     string
	}
	var rows []PushMirrorAfter
	require.NoError(t, x.Table("push_mirror").Find(&rows))
	assert.Len(t, rows, 1)
	exist, err := x.IsTableExist("push_mirror_history")
	require.NoError(t, err)
	assert.True(t, exist)
}
