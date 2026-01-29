// Copyright 2025 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package application

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestListAppsByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	apps, err := ListAppsByOwnerID(t.Context(), 1)

	assert.NoError(t, err)
	assert.Len(t, apps, 1)
	assert.Equal(t, int64(43), apps[0].ID)
}

func TestNewEmptyAppPermMap(t *testing.T) {
	permMap := NewEmptyAppPermMap()
	assert.NotNil(t, permMap)
	assert.Len(t, permMap, 4)

	for group, permList := range permMap {
		assert.NotNil(t, permList)
		assert.NotNil(t, group)

		for permItem, value := range permList {
			assert.NotNil(t, permItem)
			assert.Equal(t, PermLevelNone, value)
		}
	}
}

func TestGetAppByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	app, err := GetAppByName(t.Context(), "app1")
	assert.NoError(t, err)
	assert.Equal(t, int64(43), app.ID)
}
