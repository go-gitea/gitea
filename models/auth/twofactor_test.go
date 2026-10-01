// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"crypto/md5"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"

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
	assert.True(t, strings.HasPrefix(tfa.Secret, "pbkdf2$"))
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

func TestTwoFactorLegacySecretUpgrade(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, err := totp.Generate(totp.GenerateOpts{SecretSize: 40, Issuer: "gitea-test", AccountName: "legacy-upgrade"})
	require.NoError(t, err)
	secretStr := key.Secret()

	legacyKey := md5.Sum([]byte(setting.SecretKey))
	ciphertext, err := secret.AesEncrypt(legacyKey[:], []byte(secretStr))
	require.NoError(t, err)
	legacySecret := base64.StdEncoding.EncodeToString(ciphertext)

	const uid int64 = 1001
	tfa := &auth_model.TwoFactor{UID: uid, Secret: legacySecret}
	require.NoError(t, auth_model.NewTwoFactor(t.Context(), tfa))
	require.False(t, strings.HasPrefix(tfa.Secret, "pbkdf2$"))

	passcode, err := totp.GenerateCode(secretStr, time.Now())
	require.NoError(t, err)
	ok, err := tfa.ValidateAndConsumeTOTP(t.Context(), passcode)
	require.NoError(t, err)
	assert.True(t, ok)

	reloaded, err := auth_model.GetTwoFactorByUID(t.Context(), uid)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(reloaded.Secret, "pbkdf2$"))
	assert.NotEqual(t, legacySecret, reloaded.Secret)
	assert.Equal(t, passcode, reloaded.LastUsedPasscode)

	// after upgrade, a fresh passcode from the next TOTP window still validates from the DB row
	nextPasscode, err := totp.GenerateCode(secretStr, time.Now().Add(30*time.Second))
	require.NoError(t, err)
	ok, err = reloaded.ValidateAndConsumeTOTP(t.Context(), nextPasscode)
	require.NoError(t, err)
	assert.True(t, ok)
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

	// Both records are removed and counted separately.
	totpCount, webAuthn, err := auth_model.DisableTwoFactor(ctx, uid)
	require.NoError(t, err)
	assert.EqualValues(t, 1, totpCount)
	assert.EqualValues(t, 1, webAuthn)

	has, err = auth_model.HasTwoFactorOrWebAuthn(ctx, uid)
	require.NoError(t, err)
	assert.False(t, has)

	// A second call on a user without 2FA is a no-op.
	totpCount, webAuthn, err = auth_model.DisableTwoFactor(ctx, uid)
	require.NoError(t, err)
	assert.EqualValues(t, 0, totpCount)
	assert.EqualValues(t, 0, webAuthn)
}
