// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package algorithm

import (
	"crypto"
	"fmt"
	"hash"
)

// Hash is an official hash function algorithm. See RFC 4880, section 9.4.
type Hash interface {
	// Id returns the algorithm ID, as a byte, of Hash.
	Id() uint8
	// Available reports whether the given hash function is linked into the binary.
	Available() bool
	// HashFunc simply returns the value of h so that Hash implements SignerOpts.
	HashFunc() crypto.Hash
	// New returns a new hash.Hash calculating the given hash function. New
	// panics if the hash function is not linked into the binary.
	New() hash.Hash
	// Size returns the length, in bytes, of a digest resulting from the given
	// hash function. It doesn't require that the hash function in question be
	// linked into the program.
	Size() int
	// String is the name of the hash function corresponding to the given
	// OpenPGP hash id.
	String() string
}

// The following vars mirror the crypto/Hash supported hash functions.
var (
	MD5       Hash = cryptoHash{1, crypto.MD5}
	SHA1      Hash = cryptoHash{2, crypto.SHA1}
	RIPEMD160 Hash = cryptoHash{3, crypto.RIPEMD160}
	SHA256    Hash = cryptoHash{8, crypto.SHA256}
	SHA384    Hash = cryptoHash{9, crypto.SHA384}
	SHA512    Hash = cryptoHash{10, crypto.SHA512}
	SHA224    Hash = cryptoHash{11, crypto.SHA224}
)

// HashById represents the different hash functions specified for OpenPGP. See
// http://www.iana.org/assignments/pgp-parameters/pgp-parameters.xhtml#pgp-parameters-14
var (
	HashById = map[uint8]Hash{
		MD5.Id():       MD5,
		SHA1.Id():      SHA1,
		RIPEMD160.Id(): RIPEMD160,
		SHA256.Id():    SHA256,
		SHA384.Id():    SHA384,
		SHA512.Id():    SHA512,
		SHA224.Id():    SHA224,
	}
)

// cryptoHash contains pairs relating OpenPGP's hash identifier with
// Go's crypto.Hash type. See RFC 4880, section 9.4.
type cryptoHash struct {
	id uint8
	crypto.Hash
}

// Id returns the algorithm ID, as a byte, of cryptoHash.
func (h cryptoHash) Id() uint8 {
	return h.id
}

var hashNames = map[uint8]string{
	MD5.Id():       "MD5",
	SHA1.Id():      "SHA1",
	RIPEMD160.Id(): "RIPEMD160",
	SHA256.Id():    "SHA256",
	SHA384.Id():    "SHA384",
	SHA512.Id():    "SHA512",
	SHA224.Id():    "SHA224",
}

func (h cryptoHash) String() string {
	s, ok := hashNames[h.id]
	if !ok {
		panic(fmt.Sprintf("Unsupported hash function %d", h.id))
	}
	return s
}
