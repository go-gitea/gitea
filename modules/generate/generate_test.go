// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package generate

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeJwtSecretBase64(t *testing.T) {
	_, err := DecodeJwtSecretBase64("abcd")
	assert.ErrorContains(t, err, "invalid base64 decoded length")
	_, err = DecodeJwtSecretBase64(strings.Repeat("a", 64))
	assert.ErrorContains(t, err, "invalid base64 decoded length")

	str32 := strings.Repeat("x", 32)
	encoded32 := base64.RawURLEncoding.EncodeToString([]byte(str32))
	decoded32, err := DecodeJwtSecretBase64(encoded32)
	assert.NoError(t, err)
	assert.Equal(t, str32, string(decoded32))
}

func TestNewJwtSecretWithBase64(t *testing.T) {
	secret, encoded, err := NewJwtSecretWithBase64()
	assert.NoError(t, err)
	assert.Len(t, secret, 32)
	decoded, err := DecodeJwtSecretBase64(encoded)
	assert.NoError(t, err)
	assert.Equal(t, secret, decoded)
}
