// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemUser(t *testing.T) {
	uid, u, err := GetPossibleUserByID(t.Context(), -1)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), uid)
	assert.Equal(t, "Ghost", u.Name)
	assert.Equal(t, "ghost", u.LowerName)
	assert.True(t, u.IsGhost())

	u = GetSystemUserByName("gHost")
	require.NotNil(t, u)
	assert.Equal(t, "Ghost", u.Name)

	uid, u, err = GetPossibleUserByID(t.Context(), -2)
	require.NoError(t, err)
	assert.Equal(t, int64(-2), uid)
	assert.Equal(t, "gitea-actions", u.Name)
	assert.Equal(t, "gitea-actions", u.LowerName)

	u = GetSystemUserByName("Gitea-actionS")
	require.NotNil(t, u)
	assert.Equal(t, "Gitea Actions", u.FullName)

	uid, u, err = GetPossibleUserByID(t.Context(), 999999)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), uid)
	assert.Equal(t, "Ghost", u.Name)
}

func TestActionsUserBranding(t *testing.T) {
	defer test.MockVariableValue(&setting.Actions.BotFullName, "Acme CI")()
	defer test.MockVariableValue(&setting.Actions.BotEmail, "ci@example.com")()
	defer test.MockVariableValue(&setting.Actions.BotAvatarURL, "https://example.com/bot.png")()

	u := NewActionsUser()
	assert.Equal(t, "gitea-actions", u.Name)
	assert.Equal(t, "Acme CI", u.FullName)
	assert.Equal(t, "ci@example.com", u.Email)
	assert.Equal(t, "https://example.com/bot.png", u.AvatarLinkWithSize(t.Context(), 0))

	// other system users are not affected by the actions branding
	assert.NotEqual(t, "https://example.com/bot.png", NewDeployKeyUser().AvatarLinkWithSize(t.Context(), 0))
	assert.Empty(t, NewDeployKeyUser().Email)
}
