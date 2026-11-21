// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"crypto/sha256"
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"

	"github.com/stretchr/testify/assert"
)

func TestVerifyRunnerToken(t *testing.T) {
	verifiedRunnerTokens.Purge()
	runner := &actions_model.ActionRunner{TokenSalt: "salt", TokenHash: auth_model.HashToken("token", "salt")}
	assert.True(t, verifyRunnerToken(runner, "token"))
	assert.True(t, verifyRunnerToken(runner, "token"))
	assert.False(t, verifyRunnerToken(runner, "wrong"))

	other := &actions_model.ActionRunner{TokenSalt: "salt", TokenHash: auth_model.HashToken("other", "salt")}
	assert.False(t, verifyRunnerToken(other, "token"))
	assert.Equal(t, [][sha256.Size]byte{sha256.Sum256([]byte("token"))}, verifiedRunnerTokens.Values())
}
