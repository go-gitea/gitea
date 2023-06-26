// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	hex, err := EncryptSecret("foo", "baz")
	assert.NoError(t, err)
	str, _ := DecryptSecret("foo", hex)
	assert.Equal(t, "baz", str)

	hex, err = EncryptSecret("bar", "baz")
	assert.NoError(t, err)
	str, _ = DecryptSecret("foo", hex)
	assert.NotEqual(t, "baz", str)

	_, err = DecryptSecret("a", "b")
	assert.ErrorContains(t, err, "invalid hex string")

	_, err = DecryptSecret("a", "bb")
	assert.ErrorContains(t, err, "the key (maybe SECRET_KEY?) might be incorrect: AesDecrypt ciphertext too short")

	_, err = DecryptSecret("a", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	assert.ErrorContains(t, err, "the key (maybe SECRET_KEY?) might be incorrect: AesDecrypt invalid decrypted base64 string")
}
