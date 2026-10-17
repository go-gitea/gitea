// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package token

import (
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	_ "gitea.dev/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestDecodeTokenRejectsUserWhoCannotSignIn(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, id := range []int64{9, 37} { // user9 is inactive, user37 is prohibited from signing in
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: id})
		token, err := CreateToken(ReplyHandlerType, user, []byte{1})
		require.NoError(t, err)

		_, _, _, err = DecodeToken(t.Context(), token)
		var errToken *ErrToken
		assert.ErrorAs(t, err, &errToken)
	}
}
