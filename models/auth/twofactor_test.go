// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwoFactorValidateAndConsumeTOTP(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, err := totp.Generate(totp.GenerateOpts{SecretSize: 40, Issuer: "gitea-test", AccountName: "consume"})
	require.NoError(t, err)

	tfa := &auth_model.TwoFactor{UID: 1}
	require.NoError(t, tfa.SetSecret(key.Secret()))
	require.NoError(t, auth_model.NewTwoFactor(t.Context(), tfa))

	passcode, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	// first use of a valid passcode succeeds
	ok, err := tfa.ValidateAndConsumeTOTP(t.Context(), passcode)
	require.NoError(t, err)
	assert.True(t, ok)

	// replaying the same passcode is refused, even when still inside the TOTP validity window
	reloaded, err := auth_model.GetTwoFactorByUID(t.Context(), tfa.UID)
	require.NoError(t, err)
	ok, err = reloaded.ValidateAndConsumeTOTP(t.Context(), passcode)
	require.NoError(t, err)
	assert.False(t, ok)

	// an invalid passcode is rejected without consuming anything
	ok, err = reloaded.ValidateAndConsumeTOTP(t.Context(), "000000")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestDisableTwoFactor(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
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

	// Both records are removed and counted.
	removed, err := auth_model.DisableTwoFactor(ctx, uid)
	require.NoError(t, err)
	assert.EqualValues(t, 2, removed)

	has, err = auth_model.HasTwoFactorOrWebAuthn(ctx, uid)
	require.NoError(t, err)
	assert.False(t, has)

	// A second call on a user without 2FA is a no-op.
	removed, err = auth_model.DisableTwoFactor(ctx, uid)
	require.NoError(t, err)
	assert.EqualValues(t, 0, removed)
}
