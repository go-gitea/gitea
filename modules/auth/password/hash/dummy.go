// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"encoding/hex"
)

// DummyHasher implements PasswordHasher and is a dummy hasher that simply
// puts the password in place with its salt
// This SHOULD NOT be used in production and is provided to make the integration
// tests faster only
type DummyHasher struct{}

// HashWithSaltBytes a provided password and salt
func (hasher *DummyHasher) HashWithSaltBytes(password string, salt []byte) string {
	if hasher == nil {
		return ""
	}

	if len(salt) == 10 {
		return string(salt) + ":" + password
	}

	return hex.EncodeToString(salt) + ":" + password
}

// NewDummyHasher is a factory method to create a DummyHasher
// Any provided configuration is ignored
func NewDummyHasher(_ string) *DummyHasher {
	return &DummyHasher{}
}
