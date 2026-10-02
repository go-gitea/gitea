// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/json"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReencryptSecrets(t *testing.T) {
	type TwoFactor struct {
		ID     int64 `xorm:"pk autoincr"`
		Secret string
	}
	type Webhook struct {
		ID                           int64  `xorm:"pk autoincr"`
		HeaderAuthorizationEncrypted string `xorm:"TEXT"`
	}
	type Secret struct {
		ID   int64  `xorm:"pk autoincr"`
		Data string `xorm:"LONGTEXT"`
	}
	type LoginSource struct {
		ID   int64 `xorm:"pk autoincr"`
		Type int
		Cfg  string `xorm:"TEXT"`
	}
	type Task struct {
		ID             int64  `xorm:"pk autoincr"`
		PayloadContent string `xorm:"TEXT"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(TwoFactor), new(Webhook), new(Secret), new(LoginSource), new(Task))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	legacyEncrypt := func(key []byte, plaintext string) []byte {
		ciphertext, err := secret.AesEncrypt(key, []byte(plaintext))
		require.NoError(t, err)
		return ciphertext
	}
	legacyKey := sha256.Sum256([]byte(setting.SecretKey))
	legacyHex := func(plaintext string) string { return hex.EncodeToString(legacyEncrypt(legacyKey[:], plaintext)) }
	legacyTOTPKey := md5.Sum([]byte(setting.SecretKey))
	current, err := secret.EncryptSecret(setting.SecretKey, "current")
	require.NoError(t, err)

	_, err = x.Insert(
		&TwoFactor{Secret: base64.StdEncoding.EncodeToString(legacyEncrypt(legacyTOTPKey[:], "totp"))},
		&Secret{Data: legacyHex("secret")},
		&Secret{Data: current},
		&Secret{Data: "undecryptable"},
		&Task{PayloadContent: `{"clone_addr_encrypted":"` + legacyHex("https://example.com/repo.git") + `","auth_password_encrypted":"undecryptable","auth_token_encrypted":""}`},
	)
	require.NoError(t, err)

	require.NoError(t, ReencryptSecrets(t.Context(), x))
	require.NoError(t, ReencryptSecrets(t.Context(), x))

	column := func(table, col string) []string {
		rows, err := x.Table(table).Cols(col).OrderBy("id").QueryString()
		require.NoError(t, err)
		values := make([]string, 0, len(rows))
		for _, row := range rows {
			values = append(values, row[col])
		}
		return values
	}
	decrypt := func(encrypted string) string {
		require.True(t, strings.HasPrefix(encrypted, "v2:"))
		plaintext, err := secret.DecryptSecret(setting.SecretKey, encrypted)
		require.NoError(t, err)
		return plaintext
	}
	jsonField := func(value, field string) string {
		var obj map[string]string
		require.NoError(t, json.Unmarshal([]byte(value), &obj))
		return obj[field]
	}

	assert.Equal(t, "totp", decrypt(column("two_factor", "secret")[0]))
	secrets := column("secret", "data")
	assert.Equal(t, "secret", decrypt(secrets[0]))
	assert.Equal(t, current, secrets[1])
	assert.Equal(t, "undecryptable", secrets[2])
	payload := column("task", "payload_content")[0]
	assert.Equal(t, "https://example.com/repo.git", decrypt(jsonField(payload, "clone_addr_encrypted")))
	assert.Equal(t, "undecryptable", jsonField(payload, "auth_password_encrypted"))
	assert.Empty(t, jsonField(payload, "auth_token_encrypted"))
}
