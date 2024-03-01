// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: 3})
	expectedToken, err := GetLatestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, token, expectedToken)
}

func TestNewRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token, err := NewRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	expectedToken, err := GetLatestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, token, expectedToken)
}

func TestUpdateRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: 3})
	token.IsActive = true
	assert.NoError(t, UpdateRunnerToken(db.DefaultContext, token))
	expectedToken, err := GetLatestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, token, expectedToken)
}
