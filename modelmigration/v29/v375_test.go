// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddParentOrgIDToUser(t *testing.T) {
	type User struct {
		ID   int64 `xorm:"pk autoincr"`
		Name string
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(User))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	existing := &User{Name: "acme-root"}
	_, err := x.Insert(existing)
	require.NoError(t, err)

	require.NoError(t, AddParentOrgIDToUser(t.Context(), x))

	type MigratedUser struct {
		ID          int64 `xorm:"pk autoincr"`
		Name        string
		ParentOrgID int64 `xorm:"parent_org_id INDEX NOT NULL DEFAULT 0"`
	}
	func() {
		got := &MigratedUser{ID: existing.ID}
		has, err := x.Table("user").Get(got)
		require.NoError(t, err)
		require.True(t, has)
		assert.Equal(t, int64(0), got.ParentOrgID)
	}()

	child := &MigratedUser{Name: "acme-platform", ParentOrgID: existing.ID}
	_, err = x.Table("user").Insert(child)
	require.NoError(t, err)

	gotChild := &MigratedUser{ID: child.ID}
	has, err := x.Table("user").Get(gotChild)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, existing.ID, gotChild.ParentOrgID)
}
