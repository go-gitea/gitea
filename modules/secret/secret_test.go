// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	encrypted, err := EncryptSecret("foo", "baz")
	assert.NoError(t, err)
	str, _ := DecryptSecret("foo", encrypted)
	assert.Equal(t, "baz", str)
	str, _ = DecryptSecret("bar", encrypted)
	assert.NotEqual(t, "baz", str)

	legacyKey := sha256.Sum256([]byte("foo"))
	legacy, err := AesEncrypt(legacyKey[:], []byte("baz"))
	assert.NoError(t, err)
	str, err = DecryptSecret("foo", hex.EncodeToString(legacy))
	assert.NoError(t, err)
	assert.Equal(t, "baz", str)

	_, err = DecryptSecret("a", "b")
	assert.ErrorContains(t, err, "invalid hex string")

	_, err = DecryptSecret("a", "bb")
	assert.ErrorContains(t, err, "the key (maybe SECRET_KEY?) might be incorrect: AesDecrypt ciphertext too short")

	_, err = DecryptSecret("a", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	assert.ErrorContains(t, err, "the key (maybe SECRET_KEY?) might be incorrect: AesDecrypt invalid decrypted base64 string")
}
