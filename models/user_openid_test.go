// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserOpenIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	oids, err := GetUserOpenIDs(int64(1))
	if assert.NoError(t, err) && assert.Len(t, oids, 2) {
		assert.Equal(t, oids[0].URI, "https://user1.domain1.tld/")
		assert.False(t, oids[0].Show)
		assert.Equal(t, oids[1].URI, "http://user1.domain2.tld/")
		assert.True(t, oids[1].Show)
	}

	oids, err = GetUserOpenIDs(int64(2))
	if assert.NoError(t, err) && assert.Len(t, oids, 1) {
		assert.Equal(t, oids[0].URI, "https://domain1.tld/user2/")
		assert.True(t, oids[0].Show)
	}
}

func TestGetUserByOpenID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	_, err := GetUserByOpenID("https://unknown")
	if assert.Error(t, err) {
		assert.True(t, IsErrUserNotExist(err))
	}

	user, err := GetUserByOpenID("https://user1.domain1.tld")
	if assert.NoError(t, err) {
		assert.Equal(t, user.ID, int64(1))
	}

	user, err = GetUserByOpenID("https://domain1.tld/user2/")
	if assert.NoError(t, err) {
		assert.Equal(t, user.ID, int64(2))
	}
}

func TestToggleUserOpenIDVisibility(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	oids, err := GetUserOpenIDs(int64(2))
	if !assert.NoError(t, err) || !assert.Len(t, oids, 1) {
		return
	}
	assert.True(t, oids[0].Show)

	err = ToggleUserOpenIDVisibility(oids[0].ID)
	if !assert.NoError(t, err) {
		return
	}

	oids, err = GetUserOpenIDs(int64(2))
	if !assert.NoError(t, err) || !assert.Len(t, oids, 1) {
		return
	}
	assert.False(t, oids[0].Show)
	err = ToggleUserOpenIDVisibility(oids[0].ID)
	if !assert.NoError(t, err) {
		return
	}

	oids, err = GetUserOpenIDs(int64(2))
	if !assert.NoError(t, err) {
		return
	}
	if assert.Len(t, oids, 1) {
		assert.True(t, oids[0].Show)
	}
}
