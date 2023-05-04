// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashedBuffer(t *testing.T) {
	cases := []struct {
		MaxMemorySize int
		Data          string
		HashMD5       string
		HashSHA1      string
		HashSHA256    string
		HashSHA512    string
	}{
		{5, "test", "098f6bcd4621d373cade4e832627b4f6", "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3", "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", "ee26b0dd4af7e749aa1a8ee3c10ae9923f618980772e473f8819a5d4940e0db27ac185f8a0e1d5f84f88bc887fd67b143732c304cc5fa9ad8e6f57f50028a8ff"},
		{5, "testtest", "05a671c66aefea124cc08b76ea6d30bb", "51abb9636078defbf888d8457a7c76f85c8f114c", "37268335dd6931045bdcdf92623ff819a64244b53d0e746d438797349d4da578", "125d6d03b32c84d492747f79cf0bf6e179d287f341384eb5d6d3197525ad6be8e6df0116032935698f99a09e265073d1d6c32c274591bf1d0a20ad67cba921bc"},
	}

	for _, c := range cases {
		buf, err := CreateHashedBufferFromReaderWithSize(strings.NewReader(c.Data), c.MaxMemorySize)
		assert.NoError(t, err)

		assert.EqualValues(t, len(c.Data), buf.Size())

		data, err := io.ReadAll(buf)
		assert.NoError(t, err)
		assert.Equal(t, c.Data, string(data))

		hashMD5, hashSHA1, hashSHA256, hashSHA512 := buf.Sums()
		assert.Equal(t, c.HashMD5, hex.EncodeToString(hashMD5))
		assert.Equal(t, c.HashSHA1, hex.EncodeToString(hashSHA1))
		assert.Equal(t, c.HashSHA256, hex.EncodeToString(hashSHA256))
		assert.Equal(t, c.HashSHA512, hex.EncodeToString(hashSHA512))

		assert.NoError(t, buf.Close())
	}
}
