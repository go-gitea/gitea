// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestLookupUserRedirect(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	userID, err := user_model.LookupUserRedirect("olduser1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, userID)

	_, err = user_model.LookupUserRedirect("doesnotexist")
	assert.True(t, user_model.IsErrUserRedirectNotExist(err))
}
