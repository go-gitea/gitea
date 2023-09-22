// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"encoding/hex"
	"strings"

	"code.gitea.io/gitea/internal/modules/log"

	"golang.org/x/crypto/scrypt"
)

func init() {
	MustRegister("scrypt", NewScryptHasher)
}

// ScryptHasher implements PasswordHasher
// and uses the scrypt key derivation function.
type ScryptHasher struct {
	n, r, p, keyLen int
}

// HashWithSaltBytes a provided password and salt
func (hasher *ScryptHasher) HashWithSaltBytes(password string, salt []byte) string {
	if hasher == nil {
		return ""
	}
	hashedPassword, _ := scrypt.Key([]byte(password), salt, hasher.n, hasher.r, hasher.p, hasher.keyLen)
	return hex.EncodeToString(hashedPassword)
}

// NewScryptHasher is a factory method to create an ScryptHasher
// The provided config should be either empty or of the form:
// "<n>$<r>$<p>$<keyLen>", where <x> is the string representation
// of an integer
func NewScryptHasher(config string) *ScryptHasher {
	// This matches the original configuration for `scrypt` prior to storing hash parameters
	// in the database.
	// THESE VALUES MUST NOT BE CHANGED OR BACKWARDS COMPATIBILITY WILL BREAK
	hasher := &ScryptHasher{
		n:      1 << 16,
		r:      16,
		p:      2, // 2 passes through memory - this default config will use 128MiB in total.
		keyLen: 50,
	}

	if config == "" {
		return hasher
	}

	vals := strings.SplitN(config, "$", 4)
	if len(vals) != 4 {
		log.Error("invalid scrypt hash spec %s", config)
		return nil
	}
	var err error
	hasher.n, err = parseIntParam(vals[0], "n", "scrypt", config, nil)
	hasher.r, err = parseIntParam(vals[1], "r", "scrypt", config, err)
	hasher.p, err = parseIntParam(vals[2], "p", "scrypt", config, err)
	hasher.keyLen, err = parseIntParam(vals[3], "keyLen", "scrypt", config, err)
	if err != nil {
		return nil
	}
	return hasher
}
