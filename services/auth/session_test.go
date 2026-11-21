// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionVerify(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)

	sess := session.NewMockMemStore("dummy-sid")
	method := &Session{}

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.NoError(t, sess.Set(session.KeyUID, user.ID))
	u, err := method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, user.ID, u.ID)

	require.NoError(t, user_model.UpdateUserCols(t.Context(), &user_model.User{ID: user.ID, Type: user_model.UserTypeBot}, "type"))
	u, err = method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	assert.Nil(t, u)
}

func TestSessionVerifyGeneration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	method := &Session{}
	newSession := func(generation any) session.Store {
		sess := session.NewMockMemStore("generation-session")
		require.NoError(t, sess.Set(session.KeyUID, user.ID))
		if generation != nil {
			require.NoError(t, sess.Set(session.KeySessionGeneration, generation))
		}
		return sess
	}
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)

	t.Run("legacy session before first revocation", func(t *testing.T) {
		user.SessionGeneration = 0
		require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "session_generation"))
		verified, err := method.Verify(req, nil, nil, newSession(nil))
		require.NoError(t, err)
		assert.Equal(t, user.ID, verified.ID)
	})

	t.Run("matching generation", func(t *testing.T) {
		user.SessionGeneration = 2
		require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "session_generation"))
		verified, err := method.Verify(req, nil, nil, newSession(int64(2)))
		require.NoError(t, err)
		assert.Equal(t, user.ID, verified.ID)
	})

	t.Run("stale generation", func(t *testing.T) {
		user.SessionGeneration = 2
		require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "session_generation"))
		verified, err := method.Verify(req, nil, nil, newSession(int64(1)))
		require.NoError(t, err)
		assert.Nil(t, verified)
	})

	t.Run("missing generation after revocation", func(t *testing.T) {
		user.SessionGeneration = 2
		require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "session_generation"))
		verified, err := method.Verify(req, nil, nil, newSession(nil))
		require.NoError(t, err)
		assert.Nil(t, verified)
	})
}
