// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemUser(t *testing.T) {
	u, err := GetPossibleUserByID(db.DefaultContext, -1)
	require.NoError(t, err)
	assert.Equal(t, "Ghost", u.Name)
	assert.Equal(t, "ghost", u.LowerName)
	assert.True(t, u.IsGhost())
	assert.True(t, IsGhostUserName("gHost"))

	u, err = GetPossibleUserByID(db.DefaultContext, -2)
	require.NoError(t, err)
	assert.Equal(t, "gitea-actions", u.Name)
	assert.Equal(t, "gitea-actions", u.LowerName)
	assert.True(t, u.IsGiteaActions())
	assert.True(t, IsGiteaActionsUserName("Gitea-actionS"))

	_, err = GetPossibleUserByID(db.DefaultContext, -3)
	require.Error(t, err)
}
