// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

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

func TestNewUserRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, db.PrepareTestDatabase())

	user := db.AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	assert.NoError(t, newUserRedirect(db.GetEngine(db.DefaultContext), user.ID, user.Name, "newusername"))

	db.AssertExistsAndLoadBean(t, &UserRedirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
	db.AssertExistsAndLoadBean(t, &UserRedirect{
		LowerName:      "olduser1",
		RedirectUserID: user.ID,
	})
}

func TestNewUserRedirect2(t *testing.T) {
	// redirect to previously used name
	assert.NoError(t, db.PrepareTestDatabase())

	user := db.AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	assert.NoError(t, newUserRedirect(db.GetEngine(db.DefaultContext), user.ID, user.Name, "olduser1"))

	db.AssertExistsAndLoadBean(t, &UserRedirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
	db.AssertNotExistsBean(t, &UserRedirect{
		LowerName:      "olduser1",
		RedirectUserID: user.ID,
	})
}

func TestNewUserRedirect3(t *testing.T) {
	// redirect for a previously-unredirected user
	assert.NoError(t, db.PrepareTestDatabase())

	user := db.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.NoError(t, newUserRedirect(db.GetEngine(db.DefaultContext), user.ID, user.Name, "newusername"))

	db.AssertExistsAndLoadBean(t, &UserRedirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
}
