// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"

	"github.com/stretchr/testify/assert"
)

func TestVerifyRunnerToken(t *testing.T) {
	const uuid, token = "test-runner-uuid", "0123456789abcdef0123456789abcdef01234567"
	newRunner := func(salt string) *actions_model.ActionRunner {
		return &actions_model.ActionRunner{
			ID:        1,
			UUID:      uuid,
			TokenSalt: salt,
			TokenHash: auth_model.HashToken(token, salt),
		}
	}
	t.Cleanup(func() { auth_model.TokenCache().Purge() })
	auth_model.TokenCache().Purge()

	runner := newRunner("salt-a")
	assert.False(t, verifyRunnerToken(runner, uuid, ""))
	assert.False(t, verifyRunnerToken(runner, uuid, "wrong-token"))
	assert.Zero(t, auth_model.TokenCache().Len(), "failed verifications must not be cached")

	assert.True(t, verifyRunnerToken(runner, uuid, token))
	assert.Equal(t, 1, auth_model.TokenCache().Len())

	// cache hit: still valid even if the salt is not usable any more, because the stored hash is unchanged
	cachedRunner := *runner
	cachedRunner.TokenSalt = "different-salt"
	assert.True(t, verifyRunnerToken(&cachedRunner, uuid, token))

	// a wrong token never benefits from the cached entry of the right one
	assert.False(t, verifyRunnerToken(runner, uuid, token+"x"))

	// the runner token was regenerated: the stored hash changed, the stale cache entry must be rejected and dropped
	regenerated := newRunner("salt-b")
	regenerated.TokenHash = auth_model.HashToken("another-token", regenerated.TokenSalt)
	assert.False(t, verifyRunnerToken(regenerated, uuid, token))
	assert.Zero(t, auth_model.TokenCache().Len())
}
