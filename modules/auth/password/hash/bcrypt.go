// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"golang.org/x/crypto/bcrypt"
)

func init() {
	MustRegister("bcrypt", NewBcryptHasher)
}

// BcryptHasher implements PasswordHasher
// and uses the bcrypt password hash function.
type BcryptHasher struct {
	cost int
}

// HashWithSaltBytes a provided password and salt
func (hasher *BcryptHasher) HashWithSaltBytes(password string, salt []byte) string {
	if hasher == nil {
		return ""
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), hasher.cost)
	return string(hashedPassword)
}

func (hasher *BcryptHasher) VerifyPassword(password, hashedPassword, salt string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

// NewBcryptHasher is a factory method to create an BcryptHasher
// The provided config should be either empty or the string representation of the "<cost>"
// as an integer
func NewBcryptHasher(config string) *BcryptHasher {
	// This matches the original configuration for `bcrypt` prior to storing hash parameters
	// in the database.
	// THESE VALUES MUST NOT BE CHANGED OR BACKWARDS COMPATIBILITY WILL BREAK
	hasher := &BcryptHasher{
		cost: 10, // cost=10. i.e. 2^10 rounds of key expansion.
	}

	if config == "" {
		return hasher
	}
	var err error
	hasher.cost, err = parseIntParam(config, "cost", "bcrypt", config, nil)
	if err != nil {
		return nil
	}

	return hasher
}
