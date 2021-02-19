// Copyright 2021 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// @todo: are the swagger models needed?

// CryptBCrypt represents a BCrypt parameter set
// swagger:model
type CryptBCrypt struct {
	Cost int
}

// CryptSCrypt represents a SCrypt parameter set
// swagger:model
type CryptSCrypt struct {
	N         int
	R         int
	P         int
	KeyLength int
}

// CryptArgon2 represents a Argon2 parameter set
// swagger:model
type CryptArgon2 struct {
	Iterations  uint32
	Memory      uint32
	Parallelism uint8
	KeyLength   uint32
}

// CryptPbkdf2 represents a Pbkdf2 parameter set
// swagger:model
type CryptPbkdf2 struct {
	Iterations int
	KeyLength  int
}

var (
	// BCryptFallback stores parameters for bcrypt algo
	BCryptFallback = CryptBCrypt{
		Cost: 10,
	}

	// SCryptFallback stores parameters for scrypt algo
	SCryptFallback = CryptSCrypt{
		N:         65536,
		R:         16,
		P:         2,
		KeyLength: 50,
	}

	// Argon2Fallback stores params for argon2 algo
	Argon2Fallback = CryptArgon2{
		Iterations:  2,
		Memory:      65536,
		Parallelism: 8,
		KeyLength:   50,
	}

	// Pbkdf2Fallback stores parameters for pbkdf2 algo
	Pbkdf2Fallback = CryptPbkdf2{
		Iterations: 10000,
		KeyLength:  50,
	}
)
