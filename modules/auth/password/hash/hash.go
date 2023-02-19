// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/modules/log"
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
	Name string
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
	// The bcrypt package has its own specialized compare function that takes into
	// account the stored password's bcrypt parameters.
	if verifier, ok := algorithm.PasswordSaltHasher.(PasswordVerifier); ok {
		return verifier.VerifyPassword(providedPassword, hashedPassword, salt)
	}

	// Compute the hash of the password.
	providedPasswordHash, err := algorithm.Hash(providedPassword, salt)
	if err != nil {
		log.Error("passwordhash: %v.Hash(): %v", algorithm.Name, err)
		return false
	}

	// Compare it against the hashed password in constant-time.
	return subtle.ConstantTimeCompare([]byte(hashedPassword), []byte(providedPasswordHash)) == 1
}

var (
	lastNonDefaultAlgorithm  atomic.Value
	availableHasherFactories = map[string]func(string) PasswordSaltHasher{}
)

// Register registers a PasswordSaltHasher with the availableHasherFactories
// This is not thread safe.
func Register[T PasswordSaltHasher](name string, newFn func(config string) T) {
	if _, has := availableHasherFactories[name]; has {
		panic(fmt.Errorf("duplicate registration of password salt hasher: %s", name))
	}

	availableHasherFactories[name] = func(config string) PasswordSaltHasher {
		n := newFn(config)
		return n
	}
}

// In early versions of gitea the password hash algorithm field could be empty
// At that point the default was `pbkdf2` without configuration values
// Please note this is not the same as the DefaultAlgorithm
const defaultEmptyHashAlgorithmName = "pbkdf2"

func Parse(algorithm string) *PasswordHashAlgorithm {
	if algorithm == "" {
		algorithm = defaultEmptyHashAlgorithmName
	}

	if DefaultHashAlgorithm != nil && algorithm == DefaultHashAlgorithm.Name {
		return DefaultHashAlgorithm
	}

	ptr := lastNonDefaultAlgorithm.Load()
	if ptr != nil {
		hashAlgorithm, ok := ptr.(*PasswordHashAlgorithm)
		if ok && hashAlgorithm.Name == algorithm {
			return hashAlgorithm
		}
	}

	vals := strings.SplitN(algorithm, "$", 2)
	var name string
	var config string
	if len(vals) == 0 {
		return nil
	}
	name = vals[0]
	if len(vals) > 1 {
		config = vals[1]
	}
	newFn, has := availableHasherFactories[name]
	if !has {
		return nil
	}
	ph := newFn(config)
	if ph == nil {
		return nil
	}
	hashAlgorithm := &PasswordHashAlgorithm{
		PasswordSaltHasher: ph,
		Name:               algorithm,
	}

	lastNonDefaultAlgorithm.Store(hashAlgorithm)

	return hashAlgorithm
}
