// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"github.com/stretchr/testify/assert"
)

func TestGetLastestRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: 3})
	expected_token, err := GetLastestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, token, expected_token)
}

func TestNewRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token, err := NewRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	expected_token, err := GetLastestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, token, expected_token)
}

func TestUpdateRunnerToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	token := unittest.AssertExistsAndLoadBean(t, &ActionRunnerToken{ID: 3})
	token.IsActive = true
	assert.NoError(t, UpdateRunnerToken(db.DefaultContext, token))
	expected_token, err := GetLastestRunnerToken(db.DefaultContext, 1, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, token, expected_token)
}
