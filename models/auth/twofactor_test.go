// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"crypto/md5"
	"encoding/base64"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwoFactorSetSecretUsesPBKDF2(t *testing.T) {
	oldSecretKey := setting.SecretKey
	setting.SecretKey = "twofactor-test-secret"
	defer func() {
		setting.SecretKey = oldSecretKey
	}()

	secretStr := "JBSWY3DPEHPK3PXP"
	twofa := &auth_model.TwoFactor{}
	require.NoError(t, twofa.SetSecret(secretStr))

	assert.NotEmpty(t, twofa.SecretSalt)
	assert.Equal(t, "pbkdf2", twofa.SecretAlgo)

	passcode, err := totp.GenerateCode(secretStr, time.Now())
	require.NoError(t, err)

	ok, upgraded, err := twofa.ValidateTOTP(passcode)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, upgraded)
}

func TestTwoFactorLegacySecretUpgrade(t *testing.T) {
	oldSecretKey := setting.SecretKey
	setting.SecretKey = "twofactor-test-secret"
	defer func() {
		setting.SecretKey = oldSecretKey
	}()

	secretStr := "JBSWY3DPEHPK3PXP"
	legacyKey := md5.Sum([]byte(setting.SecretKey))
	ciphertext, err := secret.AesEncrypt(legacyKey[:], []byte(secretStr))
	require.NoError(t, err)
	legacySecret := base64.StdEncoding.EncodeToString(ciphertext)

	twofa := &auth_model.TwoFactor{Secret: legacySecret}
	passcode, err := totp.GenerateCode(secretStr, time.Now())
	require.NoError(t, err)

	ok, upgraded, err := twofa.ValidateTOTP(passcode)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, upgraded)
	assert.NotEmpty(t, twofa.SecretSalt)
	assert.Equal(t, "pbkdf2", twofa.SecretAlgo)
	assert.NotEqual(t, legacySecret, twofa.Secret)
}
