// eddsa.go - EdDSA wrappers.
// Copyright (C) 2017  Yawning Angel.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package eddsa provides EdDSA (Ed25519) wrappers.
package eddsa

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/agl/ed25519/extra25519"
	"github.com/katzenpost/core/crypto/ecdh"
	"github.com/katzenpost/core/utils"
	"golang.org/x/crypto/ed25519"
)

const (
	// PublicKeySize is the size of a serialized PublicKey in bytes (32 bytes).
	PublicKeySize = ed25519.PublicKeySize

	// PrivateKeySize is the size of a serialized PrivateKey in bytes (64 bytes).
	PrivateKeySize = ed25519.PrivateKeySize

	// SignatureSize is the size of a serialized Signature in bytes (64 bytes).
	SignatureSize = ed25519.SignatureSize

	keyType = "ed25519"
)

var errInvalidKey = errors.New("eddsa: invalid key")

// PublicKey is a EdDSA public key.
type PublicKey struct {
	pubKey    ed25519.PublicKey
	b64String string
}

// InternalPtr returns a pointer to the internal (`golang.org/x/crypto/ed25519`)
// data structure.  Most people should not use this.
func (k *PublicKey) InternalPtr() *ed25519.PublicKey {
	return &k.pubKey
}

// Bytes returns the raw public key.
func (k *PublicKey) Bytes() []byte {
	return k.pubKey
}

// Identity returns the key's identity, in this case it's our
// public key in bytes.
func (k *PublicKey) Identity() []byte {
	return k.Bytes()
}

// ByteArray returns the raw public key as an array suitable for use as a map
// key.
func (k *PublicKey) ByteArray() [PublicKeySize]byte {
	var pk [PublicKeySize]byte
	copy(pk[:], k.pubKey[:])
	return pk
}

// FromBytes deserializes the byte slice b into the PublicKey.
func (k *PublicKey) FromBytes(b []byte) error {
	if len(b) != PublicKeySize {
		return errInvalidKey
	}

	k.pubKey = make([]byte, PublicKeySize)
	copy(k.pubKey, b)
	k.rebuildB64String()
	return nil
}

// MarshalBinary implements the BinaryMarshaler interface
// defined in https://golang.org/pkg/encoding/
func (k *PublicKey) MarshalBinary() ([]byte, error) {
	return k.Bytes(), nil
}

// UnmarshalBinary implements the BinaryUnmarshaler interface
// defined in https://golang.org/pkg/encoding/
func (k *PublicKey) UnmarshalBinary(data []byte) error {
	return k.FromBytes(data)
}

// MarshalText implements the TextMarshaler interface
// defined in https://golang.org/pkg/encoding/
func (k *PublicKey) MarshalText() ([]byte, error) {
	return []byte(base64.StdEncoding.EncodeToString(k.Bytes())), nil
}

// UnmarshalText implements the TextUnmarshaler interface
// defined in https://golang.org/pkg/encoding/
func (k *PublicKey) UnmarshalText(data []byte) error {
	return k.FromString(string(data))
}

// FromString deserializes the string s into the PublicKey.
func (k *PublicKey) FromString(s string) error {
	// Try Base16 first, a correct Base64 key will never be mis-identified.
	if raw, err := hex.DecodeString(s); err == nil {
		return k.FromBytes(raw)
	}
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		return k.FromBytes(raw)
	}
	return fmt.Errorf("eddsa: key is neither Base16 nor Base64")
}

// ToPEMFile writes out the PublicKey to a PEM file at path f.
func (k *PublicKey) ToPEMFile(f string) error {
	const keyType = "ED25519 PUBLIC KEY"

	if utils.CtIsZero(k.pubKey[:]) {
		return fmt.Errorf("eddsa: attempted to serialize scrubbed key")
	}
	blk := &pem.Block{
		Type:  keyType,
		Bytes: k.Bytes(),
	}
	return ioutil.WriteFile(f, pem.EncodeToMemory(blk), 0600)
}

// ToECDH converts the PublicKey to the corresponding ecdh.PublicKey.
func (k *PublicKey) ToECDH() *ecdh.PublicKey {
	var dhBytes, dsaBytes [32]byte
	copy(dsaBytes[:], k.Bytes())
	defer utils.ExplicitBzero(dsaBytes[:])
	extra25519.PublicKeyToCurve25519(&dhBytes, &dsaBytes)
	defer utils.ExplicitBzero(dhBytes[:])
	r := new(ecdh.PublicKey)
	r.FromBytes(dhBytes[:])
	return r
}

// Reset clears the PublicKey structure such that no sensitive data is left in
// memory.  PublicKeys, despite being public may be considered sensitive in
// certain contexts (eg: if used once in path selection).
func (k *PublicKey) Reset() {
	utils.ExplicitBzero(k.pubKey)
	k.b64String = "[scrubbed]"
}

// Verify returns true iff the signature sig is valid for the message msg.
func (k *PublicKey) Verify(sig, msg []byte) bool {
	return ed25519.Verify(k.pubKey, msg, sig)
}

// String returns the public key as a base64 encoded string.
func (k *PublicKey) String() string {
	return k.b64String
}

func (k *PublicKey) rebuildB64String() {
	k.b64String = base64.StdEncoding.EncodeToString(k.Bytes())
}

// Equal returns true iff the public key is byte for byte identical.
func (k *PublicKey) Equal(cmp *PublicKey) bool {
	return subtle.ConstantTimeCompare(k.pubKey[:], cmp.pubKey[:]) == 1
}

// PrivateKey is a EdDSA private key.
type PrivateKey struct {
	pubKey  PublicKey
	privKey ed25519.PrivateKey
}

// InternalPtr returns a pointer to the internal (`golang.org/x/crypto/ed25519`)
// data structure.  Most people should not use this.
func (k *PrivateKey) InternalPtr() *ed25519.PrivateKey {
	return &k.privKey
}

// FromBytes deserializes the byte slice b into the PrivateKey.
func (k *PrivateKey) FromBytes(b []byte) error {
	if len(b) != PrivateKeySize {
		return errInvalidKey
	}

	k.privKey = make([]byte, PrivateKeySize)
	copy(k.privKey, b)
	k.pubKey.pubKey = k.privKey.Public().(ed25519.PublicKey)
	k.pubKey.rebuildB64String()
	return nil
}

// Bytes returns the raw private key.
func (k *PrivateKey) Bytes() []byte {
	return k.privKey
}

// MarshalBinary implements the BinaryMarshaler interface
// defined in https://golang.org/pkg/encoding/
func (k *PrivateKey) MarshalBinary() ([]byte, error) {
	return k.Bytes(), nil
}

// UnmarshalBinary implements the BinaryUnmarshaler interface
// defined in https://golang.org/pkg/encoding/
func (k *PrivateKey) UnmarshalBinary(data []byte) error {
	return k.FromBytes(data)
}

// Identity returns the key's identity, in this case it's our
// public key in bytes.
func (k *PrivateKey) Identity() []byte {
	return k.PublicKey().Bytes()
}

// KeyType returns the key type string,
// in this case the constant variable
// whose value is "ed25519".
func (k *PrivateKey) KeyType() string {
	return keyType
}

// ToECDH converts the PrivateKey to the corresponding ecdh.PrivateKey.
func (k *PrivateKey) ToECDH() *ecdh.PrivateKey {
	var dsaBytes [64]byte
	defer utils.ExplicitBzero(dsaBytes[:])
	copy(dsaBytes[:], k.Bytes())

	var dhBytes [32]byte
	extra25519.PrivateKeyToCurve25519(&dhBytes, &dsaBytes)
	defer utils.ExplicitBzero(dhBytes[:])

	r := new(ecdh.PrivateKey)
	r.FromBytes(dhBytes[:])
	return r
}

// Reset clears the PrivateKey structure such that no sensitive data is left
// in memory.
func (k *PrivateKey) Reset() {
	k.pubKey.Reset()
	utils.ExplicitBzero(k.privKey)
}

// PublicKey returns the PublicKey corresponding to the PrivateKey.
func (k *PrivateKey) PublicKey() *PublicKey {
	return &k.pubKey
}

// Sign signs the message msg with the PrivateKey and returns the signature.
func (k *PrivateKey) Sign(msg []byte) []byte {
	return ed25519.Sign(k.privKey, msg)
}

// NewKeypair generates a new PrivateKey sampled from the provided entropy
// source.
func NewKeypair(r io.Reader) (*PrivateKey, error) {
	pubKey, privKey, err := ed25519.GenerateKey(r)
	if err != nil {
		return nil, err
	}

	k := new(PrivateKey)
	k.privKey = privKey
	k.pubKey.pubKey = pubKey
	k.pubKey.rebuildB64String()
	return k, nil
}

// Load loads a new PrivateKey from the PEM encoded file privFile, optionally
// creating and saving a PrivateKey instead if an entropy source is provided.
// If pubFile is specified and a key has been created, the corresponding
// PublicKey will be written to pubFile in PEM format.
func Load(privFile, pubFile string, r io.Reader) (*PrivateKey, error) {
	const keyType = "ED25519 PRIVATE KEY"

	if buf, err := ioutil.ReadFile(privFile); err == nil {
		defer utils.ExplicitBzero(buf)
		blk, rest := pem.Decode(buf)
		defer utils.ExplicitBzero(blk.Bytes)
		if len(rest) != 0 {
			return nil, fmt.Errorf("trailing garbage after PEM encoded private key")
		}
		if blk.Type != keyType {
			return nil, fmt.Errorf("invalid PEM Type: '%v'", blk.Type)
		}
		k := new(PrivateKey)
		return k, k.FromBytes(blk.Bytes)
	} else if !os.IsNotExist(err) || r == nil {
		return nil, err
	}

	k, err := NewKeypair(r)
	if err != nil {
		return nil, err
	}
	blk := &pem.Block{
		Type:  keyType,
		Bytes: k.Bytes(),
	}
	if err = ioutil.WriteFile(privFile, pem.EncodeToMemory(blk), 0600); err != nil {
		return nil, err
	}
	if pubFile != "" {
		err = k.PublicKey().ToPEMFile(pubFile)
	}
	return k, err
}
