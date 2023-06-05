// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	expectedMD5    = "e3bef03c5f3b7f6b3ab3e3053ed71e9c"
	expectedSHA1   = "060b3b99f88e96085b4a68e095bc9e3d1d91e1bc"
	expectedSHA256 = "6ccce4863b70f258d691f59609d31b4502e1ba5199942d3bc5d35d17a4ce771d"
	expectedSHA512 = "7f70e439ba8c52025c1f06cdf6ae443c4b8ed2e90059cdb9bbbf8adf80846f185a24acca9245b128b226d61753b0d7ed46580a69c8999eeff3bc13a4d0bd816c"
)

func TestMultiHasherSums(t *testing.T) {
	t.Run("Sums", func(t *testing.T) {
		h := NewMultiHasher()
		h.Write([]byte("gitea"))

		hashMD5, hashSHA1, hashSHA256, hashSHA512 := h.Sums()

		assert.Equal(t, expectedMD5, hex.EncodeToString(hashMD5))
		assert.Equal(t, expectedSHA1, hex.EncodeToString(hashSHA1))
		assert.Equal(t, expectedSHA256, hex.EncodeToString(hashSHA256))
		assert.Equal(t, expectedSHA512, hex.EncodeToString(hashSHA512))
	})

	t.Run("State", func(t *testing.T) {
		h := NewMultiHasher()
		h.Write([]byte("git"))

		state, err := h.MarshalBinary()
		assert.NoError(t, err)

		h2 := NewMultiHasher()
		err = h2.UnmarshalBinary(state)
		assert.NoError(t, err)

		h2.Write([]byte("ea"))

		hashMD5, hashSHA1, hashSHA256, hashSHA512 := h2.Sums()

		assert.Equal(t, expectedMD5, hex.EncodeToString(hashMD5))
		assert.Equal(t, expectedSHA1, hex.EncodeToString(hashSHA1))
		assert.Equal(t, expectedSHA256, hex.EncodeToString(hashSHA256))
		assert.Equal(t, expectedSHA512, hex.EncodeToString(hashSHA512))
	})
}
