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
	type testCase struct {
		id  int64
		err string
	}
	testCases := []testCase{
		{2, ""},
		{3, "not a doer"},
		{9, "inactive"},
		{37, "prohibited"},
	}
	for _, tc := range testCases {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.id})
		token, err := CreateToken(ReplyHandlerType, user, []byte{1})
		require.NoError(t, err)

		_, _, _, err = DecodeToken(t.Context(), token)
		if tc.err == "" {
			assert.NoError(t, err, "user ID %d, err: %v", tc.id, err)
		} else {
			assert.ErrorContains(t, err, tc.err, "user ID %d, err: %v", tc.id, err)
		}
	}
}
