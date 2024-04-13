// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestGetUserOpenIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	oids, err := user_model.GetUserOpenIDs(db.DefaultContext, int64(1))
	if assert.NoError(t, err) && assert.Len(t, oids, 2) {
		assert.Equal(t, "https://user1.domain1.tld/", oids[0].URI)
		assert.False(t, oids[0].Show)
		assert.Equal(t, "http://user1.domain2.tld/", oids[1].URI)
		assert.True(t, oids[1].Show)
	}

	oids, err = user_model.GetUserOpenIDs(db.DefaultContext, int64(2))
	if assert.NoError(t, err) && assert.Len(t, oids, 1) {
		assert.Equal(t, "https://domain1.tld/user2/", oids[0].URI)
		assert.True(t, oids[0].Show)
	}
}

func TestToggleUserOpenIDVisibility(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	oids, err := user_model.GetUserOpenIDs(db.DefaultContext, int64(2))
	if !assert.NoError(t, err) || !assert.Len(t, oids, 1) {
		return
	}
	assert.True(t, oids[0].Show)

	err = user_model.ToggleUserOpenIDVisibility(db.DefaultContext, oids[0].ID)
	if !assert.NoError(t, err) {
		return
	}

	oids, err = user_model.GetUserOpenIDs(db.DefaultContext, int64(2))
	if !assert.NoError(t, err) || !assert.Len(t, oids, 1) {
		return
	}
	assert.False(t, oids[0].Show)
	err = user_model.ToggleUserOpenIDVisibility(db.DefaultContext, oids[0].ID)
	if !assert.NoError(t, err) {
		return
	}

	oids, err = user_model.GetUserOpenIDs(db.DefaultContext, int64(2))
	if !assert.NoError(t, err) {
		return
	}
	if assert.Len(t, oids, 1) {
		assert.True(t, oids[0].Show)
	}
}
