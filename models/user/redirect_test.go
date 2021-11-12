// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"github.com/stretchr/testify/assert"
)

func TestLookupUserRedirect(t *testing.T) {
	assert.NoError(t, db.PrepareTestDatabase())

	userID, err := LookupUserRedirect("olduser1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, userID)

	_, err = LookupUserRedirect("doesnotexist")
	assert.True(t, IsErrUserRedirectNotExist(err))
}
