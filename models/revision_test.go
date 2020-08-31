// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRevision_LoadUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	rev := AssertExistsAndLoadBean(t, &Revision{ID: 1}).(*Revision)
	assert.NoError(t, rev.LoadUser())
	assert.NotNil(t, rev.User)
	assert.Equal(t, rev.UserID, rev.User.ID)
}

func TestRevision_LoadUser_Deleted(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	rev := AssertExistsAndLoadBean(t, &Revision{ID: 2}).(*Revision)
	assert.Nil(t, rev.LoadUser())
	assert.NotNil(t, rev.User)
	assert.Equal(t, int64(-1), rev.User.ID)
	assert.Equal(t, "Ghost", rev.User.Name)
}

func TestRevision_LoadPullRequest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	rev := AssertExistsAndLoadBean(t, &Revision{ID: 1}).(*Revision)
	assert.NoError(t, rev.LoadPullRequest())
	assert.NotNil(t, rev.PR)
	assert.Equal(t, rev.PRID, rev.PR.ID)
}
