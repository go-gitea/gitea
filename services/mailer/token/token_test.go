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
	for id, expectedErr := range map[int64]string{2: "", 3: "not a doer", 9: "inactive", 37: "prohibited"} {
		token, err := CreateToken(ReplyHandlerType, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: id}), []byte{1})
		require.NoError(t, err)
		_, _, _, err = DecodeToken(t.Context(), token)
		if expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.ErrorContains(t, err, expectedErr)
		}
	}
}
