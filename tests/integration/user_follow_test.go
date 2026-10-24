// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserFollowBot makes sure that following a bot user (an agent) from its profile page answers
// with the profile card like any individual user, instead of failing with an unsupported user type.
func TestUserFollowBot(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	bot := &user_model.User{
		Name:  "follow-bot",
		Email: "follow-bot@example.com",
		Type:  user_model.UserTypeBot,
	}
	require.NoError(t, user_model.CreateUser(t.Context(), bot, &user_model.Meta{}))

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	session := loginUser(t, doer.Name)

	req := NewRequestWithValues(t, "POST", "/"+bot.Name, map[string]string{"action": "follow"})
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `id="profile-avatar-card"`)
	assert.True(t, user_model.IsFollowing(t.Context(), doer.ID, bot.ID))

	req = NewRequestWithValues(t, "POST", "/"+bot.Name, map[string]string{"action": "unfollow"})
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `id="profile-avatar-card"`)
	assert.False(t, user_model.IsFollowing(t.Context(), doer.ID, bot.ID))
}
