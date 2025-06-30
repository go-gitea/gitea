// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"os"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
	os.Exit(m.Run())
}

func TestInitToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("NoToken", func(t *testing.T) {
		_, _ = db.Exec(db.DefaultContext, "DELETE FROM action_runner_token")
		t.Setenv("GITEA_RUNNER_REGISTRATION_TOKEN", "")
		t.Setenv("GITEA_RUNNER_REGISTRATION_TOKEN_FILE", "")
		err := initGlobalRunnerToken(db.DefaultContext)
		require.NoError(t, err)
		notEmpty, err := db.IsTableNotEmpty(&actions_model.ActionRunnerToken{})
		require.NoError(t, err)
		assert.False(t, notEmpty)
	})

	t.Run("EnvToken", func(t *testing.T) {
		tokenValue, _ := util.CryptoRandomString(32)
		t.Setenv("GITEA_RUNNER_REGISTRATION_TOKEN", tokenValue)
		t.Setenv("GITEA_RUNNER_REGISTRATION_TOKEN_FILE", "")
		err := initGlobalRunnerToken(db.DefaultContext)
		require.NoError(t, err)
		token := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunnerToken{Token: tokenValue})
		assert.True(t, token.IsActive)

		// init with the same token again, should not create a new token
		err = initGlobalRunnerToken(db.DefaultContext)
		require.NoError(t, err)
		token2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunnerToken{Token: tokenValue})
		assert.Equal(t, token.ID, token2.ID)
		assert.True(t, token.IsActive)
	})

	t.Run("EnvFileToken", func(t *testing.T) {
		tokenValue, _ := util.CryptoRandomString(32)
		f := t.TempDir() + "/token"
		_ = os.WriteFile(f, []byte(tokenValue), 0o644)
		t.Setenv("GITEA_RUNNER_REGISTRATION_TOKEN", "")
		t.Setenv("GITEA_RUNNER_REGISTRATION_TOKEN_FILE", f)
		err := initGlobalRunnerToken(db.DefaultContext)
		require.NoError(t, err)
		token := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunnerToken{Token: tokenValue})
		assert.True(t, token.IsActive)

		// if the env token is invalidated by another new token, then it shouldn't be active anymore
		_, err = actions_model.NewRunnerToken(db.DefaultContext, 0, 0)
		require.NoError(t, err)
		token = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunnerToken{Token: tokenValue})
		assert.False(t, token.IsActive)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		t.Setenv("GITEA_RUNNER_REGISTRATION_TOKEN", "abc")
		err := initGlobalRunnerToken(db.DefaultContext)
		assert.ErrorContains(t, err, "must be at least")
	})
}
