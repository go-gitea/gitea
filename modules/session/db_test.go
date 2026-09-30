// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBStoreUnchangedReleaseRefreshesExpiryWithoutRevertingChanges(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	provider := &DBProvider{}
	require.NoError(t, provider.Init(3600, ""))

	created, err := provider.Read("sid")
	require.NoError(t, err)
	require.NoError(t, created.Set("uid", 1))
	require.NoError(t, created.Release())

	reader, err := provider.Read("sid")
	require.NoError(t, err)
	writer, err := provider.Read("sid")
	require.NoError(t, err)
	require.NoError(t, writer.Set("key", "value"))
	require.NoError(t, writer.Release())
	_, err = db.GetEngine(t.Context()).ID("sid").Cols("expiry").Update(&auth_model.Session{Expiry: 1})
	require.NoError(t, err)
	require.NoError(t, reader.Delete("missing"))
	require.NoError(t, reader.Release())

	stored, err := provider.Read("sid")
	require.NoError(t, err)
	assert.Equal(t, "value", stored.Get("key"))
}
