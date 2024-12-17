// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: 3})
	expectedToken, err := GetLatestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedToken, token)
}

func TestNewRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token, err := NewRunnerToken(db.DefaultContext, 1, 0, "")
	assert.NoError(t, err)
	expectedToken, err := GetLatestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedToken, token)

	predefinedToken, err := util.CryptoRandomString(40)
	assert.NoError(t, err)

	token, err = NewRunnerToken(db.DefaultContext, 1, 0, predefinedToken)
	assert.NoError(t, err)
	assert.EqualValues(t, predefinedToken, token.Token)

	expectedToken, err = GetLatestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedToken, token)

	_, err = NewRunnerToken(db.DefaultContext, 1, 0, "invalid-token")
	assert.Error(t, err)
}

func TestUpdateRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: 3})
	token.IsActive = true
	assert.NoError(t, UpdateRunnerToken(db.DefaultContext, token))
	expectedToken, err := GetLatestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedToken, token)
}
