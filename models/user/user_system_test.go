// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemUser(t *testing.T) {
	u, err := GetPossibleUserByID(t.Context(), -1)
	require.NoError(t, err)
	assert.Equal(t, "Ghost", u.Name)
	assert.Equal(t, "ghost", u.LowerName)
	assert.True(t, u.IsGhost())

	u = GetSystemUserByName("gHost")
	require.NotNil(t, u)
	assert.Equal(t, "Ghost", u.Name)

	u, err = GetPossibleUserByID(t.Context(), -2)
	require.NoError(t, err)
	assert.Equal(t, "gitea-actions", u.Name)
	assert.Equal(t, "gitea-actions", u.LowerName)
	assert.True(t, u.IsGiteaActions())

	u = GetSystemUserByName("Gitea-actionS")
	require.NotNil(t, u)
	assert.Equal(t, "Gitea Actions", u.FullName)

	_, err = GetPossibleUserByID(t.Context(), -3)
	require.Error(t, err)
}
