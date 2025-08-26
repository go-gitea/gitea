// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestUserHasPubkeys(t *testing.T) {
	ctx := t.Context()

	t.Run("AllowUserWithGPGKey", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 36}) // user has GPG key
		hasKeys, err := userHasPubkeys(ctx, user)
		assert.NoError(t, err)
		assert.True(t, hasKeys)
	})

	t.Run("AllowUserWithSSHKey", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // user has SSH key
		hasKeys, err := userHasPubkeys(ctx, user)
		assert.NoError(t, err)
		assert.True(t, hasKeys)
	})

	t.Run("DenyUserWithNoKeys", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		hasKeys, err := userHasPubkeys(ctx, user)
		assert.NoError(t, err)
		assert.False(t, hasKeys)
	})
}
