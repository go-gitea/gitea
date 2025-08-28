// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestIsFollowing(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.True(t, user_model.IsFollowing(t.Context(), 4, 2))
	assert.False(t, user_model.IsFollowing(t.Context(), 2, 4))
	assert.False(t, user_model.IsFollowing(t.Context(), 5, unittest.NonexistentID))
	assert.False(t, user_model.IsFollowing(t.Context(), unittest.NonexistentID, 5))
	assert.False(t, user_model.IsFollowing(t.Context(), unittest.NonexistentID, unittest.NonexistentID))
}
