// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDummyHasher(t *testing.T) {
	dummy := &PasswordHashAlgorithm{
		PasswordSaltHasher: NewDummyHasher(""),
		Specification:      "dummy",
	}

	password, salt := "password", "ZogKvWdyEx"

	hash, err := dummy.Hash(password, salt)
	assert.NoError(t, err)
	assert.Equal(t, hash, salt+":"+password)

	assert.True(t, dummy.VerifyPassword(password, hash, salt))
}
