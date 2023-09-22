// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/internal/modules/log"
)

// This package takes care of hashing passwords, verifying passwords, defining
// available password algorithms, defining recommended password algorithms and
// choosing the default password algorithm.

// PasswordSaltHasher will hash a provided password with the provided saltBytes
type PasswordSaltHasher interface {
	HashWithSaltBytes(password string, saltBytes []byte) string
}

// PasswordHasher will hash a provided password with the salt
type PasswordHasher interface {
	Hash(password, salt string) (string, error)
}

// PasswordVerifier will ensure that a providedPassword matches the hashPassword when hashed with the salt
type PasswordVerifier interface {
	VerifyPassword(providedPassword, hashedPassword, salt string) bool
}

// PasswordHashAlgorithms are named PasswordSaltHashers with a default verifier and hash function
type PasswordHashAlgorithm struct {
	PasswordSaltHasher
	Specification string // The specification that is used to create the internal PasswordSaltHasher
}

// Hash the provided password with the salt and return the hash
func (algorithm *PasswordHashAlgorithm) Hash(password, salt string) (string, error) {
	var saltBytes []byte

	// There are two formats for the salt value:
	// * The new format is a (32+)-byte hex-encoded string
	// * The old format was a 10-byte binary format
	// We have to tolerate both here.
	if len(salt) == 10 {
		saltBytes = []byte(salt)
	} else {
		var err error
		saltBytes, err = hex.DecodeString(salt)
		if err != nil {
			return "", err
		}
	}

	return algorithm.HashWithSaltBytes(password, saltBytes), nil
}

// Verify the provided password matches the hashPassword when hashed with the salt
func (algorithm *PasswordHashAlgorithm) VerifyPassword(providedPassword, hashedPassword, salt string) bool {
	// Some PasswordSaltHashers have their own specialised compare function that takes into
	// account the stored parameters within the hash. e.g. bcrypt
	if verifier, ok := algorithm.PasswordSaltHasher.(PasswordVerifier); ok {
		return verifier.VerifyPassword(providedPassword, hashedPassword, salt)
	}

	// Compute the hash of the password.
	providedPasswordHash, err := algorithm.Hash(providedPassword, salt)
	if err != nil {
		log.Error("passwordhash: %v.Hash(): %v", algorithm.Specification, err)
		return false
	}

	// Compare it against the hashed password in constant-time.
	return subtle.ConstantTimeCompare([]byte(hashedPassword), []byte(providedPasswordHash)) == 1
}

var (
	lastNonDefaultAlgorithm  atomic.Value
	availableHasherFactories = map[string]func(string) PasswordSaltHasher{}
)

// MustRegister registers a PasswordSaltHasher with the availableHasherFactories
// Caution: This is not thread safe.
func MustRegister[T PasswordSaltHasher](name string, newFn func(config string) T) {
	if err := Register(name, newFn); err != nil {
		panic(err)
	}
}

// Register registers a PasswordSaltHasher with the availableHasherFactories
// Caution: This is not thread safe.
func Register[T PasswordSaltHasher](name string, newFn func(config string) T) error {
	if _, has := availableHasherFactories[name]; has {
		return fmt.Errorf("duplicate registration of password salt hasher: %s", name)
	}

	availableHasherFactories[name] = func(config string) PasswordSaltHasher {
		n := newFn(config)
		return n
	}
	return nil
}

// In early versions of gitea the password hash algorithm field of a user could be
// empty. At that point the default was `pbkdf2` without configuration values
//
// Please note this is not the same as the DefaultAlgorithm which is used
// to determine what an empty PASSWORD_HASH_ALGO setting in the app.ini means.
// These are not the same even if they have the same apparent value and they mean different things.
//
// DO NOT COALESCE THESE VALUES
const defaultEmptyHashAlgorithmSpecification = "pbkdf2"

// Parse will convert the provided algorithm specification in to a PasswordHashAlgorithm
// If the provided specification matches the DefaultHashAlgorithm Specification it will be
// used.
// In addition the last non-default hasher will be cached to help reduce the load from
// parsing specifications.
//
// NOTE: No de-aliasing is done in this function, thus any specification which does not
// contain a configuration will use the default values for that hasher. These are not
// necessarily the same values as those obtained by dealiasing. This allows for
// seamless backwards compatibility with the original configuration.
//
// To further labour this point, running `Parse("pbkdf2")` does not obtain the
// same algorithm as setting `PASSWORD_HASH_ALGO=pbkdf2` in app.ini, nor is it intended to.
// A user that has `password_hash_algo='pbkdf2'` in the db means get the original, unconfigured algorithm
// Users will be migrated automatically as they log-in to have the complete specification stored
// in their `password_hash_algo` fields by other code.
func Parse(algorithmSpec string) *PasswordHashAlgorithm {
	if algorithmSpec == "" {
		algorithmSpec = defaultEmptyHashAlgorithmSpecification
	}

	if DefaultHashAlgorithm != nil && algorithmSpec == DefaultHashAlgorithm.Specification {
		return DefaultHashAlgorithm
	}

	ptr := lastNonDefaultAlgorithm.Load()
	if ptr != nil {
		hashAlgorithm, ok := ptr.(*PasswordHashAlgorithm)
		if ok && hashAlgorithm.Specification == algorithmSpec {
			return hashAlgorithm
		}
	}

	// Now convert the provided specification in to a hasherType +/- some configuration parameters
	vals := strings.SplitN(algorithmSpec, "$", 2)
	var hasherType string
	var config string

	if len(vals) == 0 {
		// This should not happen as algorithmSpec should not be empty
		// due to it being assigned to defaultEmptyHashAlgorithmSpecification above
		// but we should be absolutely cautious here
		return nil
	}

	hasherType = vals[0]
	if len(vals) > 1 {
		config = vals[1]
	}

	newFn, has := availableHasherFactories[hasherType]
	if !has {
		// unknown hasher type
		return nil
	}

	ph := newFn(config)
	if ph == nil {
		// The provided configuration is likely invalid - it will have been logged already
		// but we cannot hash safely
		return nil
	}

	hashAlgorithm := &PasswordHashAlgorithm{
		PasswordSaltHasher: ph,
		Specification:      algorithmSpec,
	}

	lastNonDefaultAlgorithm.Store(hashAlgorithm)

	return hashAlgorithm
}
