// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"golang.org/x/crypto/pbkdf2"
)

func init() {
	MustRegister("pbkdf2", NewPBKDF2Hasher)
}

// PBKDF2Hasher implements PasswordHasher
// and uses the PBKDF2 key derivation function.
type PBKDF2Hasher struct {
	iter, keyLen int
}

// HashWithSaltBytes a provided password and salt
func (hasher *PBKDF2Hasher) HashWithSaltBytes(password string, salt []byte) string {
	if hasher == nil {
		return ""
	}
	return hex.EncodeToString(pbkdf2.Key([]byte(password), salt, hasher.iter, hasher.keyLen, sha256.New))
}

// NewPBKDF2Hasher is a factory method to create an PBKDF2Hasher
// config should be either empty or of the form:
// "<iter>$<keyLen>", where <x> is the string representation
// of an integer
func NewPBKDF2Hasher(config string) *PBKDF2Hasher {
	// This default configuration uses the following parameters:
	// iter=10000, keyLen=50.
	// This matches the original configuration for `pbkdf2` prior to storing parameters
	// in the database.
	// THESE VALUES MUST NOT BE CHANGED OR BACKWARDS COMPATIBILITY WILL BREAK
	hasher := &PBKDF2Hasher{
		iter:   10_000,
		keyLen: 50,
	}

	if config == "" {
		return hasher
	}

	vals := strings.SplitN(config, "$", 2)
	if len(vals) != 2 {
		log.Error("invalid pbkdf2 hash spec %s", config)
		return nil
	}

	var err error
	hasher.iter, err = parseIntParam(vals[0], "iter", "pbkdf2", config, nil)
	hasher.keyLen, err = parseIntParam(vals[1], "keyLen", "pbkdf2", config, err)
	if err != nil {
		return nil
	}

	return hasher
}
