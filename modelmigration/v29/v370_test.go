// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddCodespaceTables(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0)
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}
	require.NoError(t, AddCodespaceTables(t.Context(), x))
	instance := &codespace{UserID: 1, SecretsEncrypted: "instance-ciphertext"}
	_, err := x.Insert(instance)
	require.NoError(t, err)
	var loaded codespace
	found, err := x.ID(instance.ID).Get(&loaded)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, instance.SecretsEncrypted, loaded.SecretsEncrypted)

	type CodespaceUserSecret struct {
		ID            int64
		UserID        int64
		Name          string
		DataEncrypted string
	}
	secret := &CodespaceUserSecret{UserID: 1, Name: "TOKEN", DataEncrypted: "personal-ciphertext"}
	_, err = x.Insert(secret)
	require.NoError(t, err)
	var saved CodespaceUserSecret
	found, err = x.ID(secret.ID).Get(&saved)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, *secret, saved)
}
