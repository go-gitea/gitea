// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	auth_model "gitea.dev/models/auth"
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
	require.NoError(t, sess.Set(session.KeySessionGeneration, user.SessionGeneration))
	u, err := method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, user.ID, u.ID)

	require.NoError(t, user_model.UpdateUserCols(t.Context(), &user_model.User{ID: user.ID, Type: user_model.UserTypeBot}, "type"))
	u, err = method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	assert.Nil(t, u)
}

func TestSessionVerifyAndRevoke(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	otherUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	_, _, err := CreateAuthTokenForUserID(t.Context(), user.ID)
	require.NoError(t, err)
	_, _, err = CreateAuthTokenForUserID(t.Context(), otherUser.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	sessAuth := &Session{}

	sess := session.NewMockMemStore("dummy-sid")
	require.NoError(t, sess.Set(session.KeyUID, user.ID))
	require.NoError(t, sess.Set(session.KeySessionGeneration, user.SessionGeneration))

	u, err := sessAuth.Verify(req, resp, nil, sess)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, user.ID, u.ID)

	// Revoking sessions bumps the user's generation and deletes their auth tokens.
	require.NoError(t, user_model.RevokeUserSessions(t.Context(), user.ID))
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
	assert.EqualValues(t, 1, user.SessionGeneration)
	unittest.AssertNotExistsBean(t, &auth_model.AuthToken{UserID: user.ID})
	unittest.AssertExistsAndLoadBean(t, &auth_model.AuthToken{UserID: otherUser.ID})

	// The old session generation is no longer accepted.
	u, err = sessAuth.Verify(req, resp, nil, sess)
	require.NoError(t, err)
	assert.Nil(t, u)

	// A session with the updated generation succeeds.
	require.NoError(t, sess.Set(session.KeySessionGeneration, user.SessionGeneration))
	u, err = sessAuth.Verify(req, resp, nil, sess)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, user.ID, u.ID)

	// Impersonating another user and exiting restores the original user's session generation.
	require.NoError(t, ImpersonateUser(sess, otherUser))
	u, err = sessAuth.Verify(req, resp, nil, sess)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, otherUser.ID, u.ID)

	// Revoking the impersonator's sessions also invalidates their active impersonation session.
	require.NoError(t, user_model.RevokeUserSessions(t.Context(), user.ID))
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
	assert.EqualValues(t, 2, user.SessionGeneration)
	u, err = sessAuth.Verify(req, resp, nil, sess)
	require.NoError(t, err)
	assert.Nil(t, u)

	// Exiting impersonation restores the original user's session data (which has the older generation until re-authenticated).
	exited, err := ExitImpersonatedUser(sess)
	require.NoError(t, err)
	assert.True(t, exited)
	assert.Nil(t, sess.Get(session.KeyUserHasTwoFactorAuth))
	u, err = sessAuth.Verify(req, resp, nil, sess)
	require.NoError(t, err)
	assert.Nil(t, u)

	require.NoError(t, sess.Set(session.KeySessionGeneration, user.SessionGeneration))
	u, err = sessAuth.Verify(req, resp, nil, sess)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, user.ID, u.ID)
}
