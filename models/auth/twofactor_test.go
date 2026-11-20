// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/cache"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwoFactorLifecycle(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("TwoFactorValidateAndConsumeTOTP", func(t *testing.T) {
		key, err := totp.Generate(totp.GenerateOpts{SecretSize: 40, Issuer: "gitea-test", AccountName: "consume"})
		require.NoError(t, err)

		tfa := &auth_model.TwoFactor{UID: 1}
		require.NoError(t, tfa.SetSecret(key.Secret()))
		require.NoError(t, auth_model.NewTwoFactor(t.Context(), tfa))

		passcode, err := totp.GenerateCode(key.Secret(), time.Now())
		require.NoError(t, err)

		// first use of a valid passcode succeeds
		result, err := tfa.ValidateAndConsumeTOTP(t.Context(), passcode)
		require.NoError(t, err)
		assert.Equal(t, auth_model.TwoFactorResultValid, result)

		// replaying the same passcode is refused, even when still inside the TOTP validity window
		reloaded, err := auth_model.GetTwoFactorByUID(t.Context(), tfa.UID)
		require.NoError(t, err)
		result, err = reloaded.ValidateAndConsumeTOTP(t.Context(), passcode)
		require.NoError(t, err)
		assert.Equal(t, auth_model.TwoFactorResultInvalid, result)

		var limited atomic.Int32
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				result, err := reloaded.ValidateAndConsumeTOTP(t.Context(), "0000000")
				assert.NoError(t, err)
				if result == auth_model.TwoFactorResultTooManyAttempts {
					limited.Add(1)
				} else {
					assert.Equal(t, auth_model.TwoFactorResultInvalid, result)
				}
			})
		}
		wg.Wait()
		assert.EqualValues(t, 6, limited.Load())
		nextPasscode, err := totp.GenerateCode(key.Secret(), time.Now().Add(30*time.Second))
		require.NoError(t, err)
		result, err = reloaded.ValidateAndConsumeTOTP(t.Context(), nextPasscode)
		require.NoError(t, err)
		assert.Equal(t, auth_model.TwoFactorResultTooManyAttempts, result)

		cache.Remove(fmt.Sprintf("twofa_attempts_%d", tfa.UID))
		result, err = reloaded.ValidateAndConsumeTOTP(t.Context(), nextPasscode)
		require.NoError(t, err)
		assert.Equal(t, auth_model.TwoFactorResultValid, result)
	})

	t.Run("DisableTwoFactor", func(t *testing.T) {
		ctx := t.Context()

		const uid = 1000 // a uid with no user/2FA fixtures

		// Enroll TOTP and register a WebAuthn credential.
		tfa := &auth_model.TwoFactor{UID: uid}
		require.NoError(t, tfa.SetSecret("test-secret"))
		require.NoError(t, auth_model.NewTwoFactor(ctx, tfa))
		_, err := auth_model.CreateCredential(ctx, uid, "test-key", &webauthn.Credential{ID: []byte("test-cred-id")})
		require.NoError(t, err)

		has, err := auth_model.HasTwoFactorOrWebAuthn(ctx, uid)
		require.NoError(t, err)
		require.True(t, has)

		// Both records are removed and counted separately.
		totp, webAuthn, err := auth_model.DisableTwoFactor(ctx, uid)
		require.NoError(t, err)
		assert.EqualValues(t, 1, totp)
		assert.EqualValues(t, 1, webAuthn)

		has, err = auth_model.HasTwoFactorOrWebAuthn(ctx, uid)
		require.NoError(t, err)
		assert.False(t, has)

		// A second call on a user without 2FA is a no-op.
		totp, webAuthn, err = auth_model.DisableTwoFactor(ctx, uid)
		require.NoError(t, err)
		assert.EqualValues(t, 0, totp)
		assert.EqualValues(t, 0, webAuthn)
	})
}
