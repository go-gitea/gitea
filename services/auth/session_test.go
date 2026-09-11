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

	// an individual keeps its session
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.NoError(t, sess.Set(session.KeyUID, user.ID))
	u, err := method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, user.ID, u.ID)

	// a session that outlived the conversion of its user into a bot is not accepted anymore
	require.NoError(t, user_model.UpdateUserCols(t.Context(), &user_model.User{ID: user.ID, Type: user_model.UserTypeBot}, "type"))
	u, err = method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	assert.Nil(t, u)
}
