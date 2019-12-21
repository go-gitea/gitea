// ecdh.go - ECDH wrappers.
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

// Package ecdh provides ECDH (X25519) wrappers.
package ecdh

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

	"github.com/katzenpost/core/utils"
	"golang.org/x/crypto/curve25519"
)

const (
	// GroupElementLength is the length of a ECDH group element in bytes.
	GroupElementLength = 32

	// PublicKeySize is the size of a serialized PublicKey in bytes.
	PublicKeySize = GroupElementLength

	// PrivateKeySize is the size of a serialized PrivateKey in bytes.
	PrivateKeySize = GroupElementLength
)

var errInvalidKey = errors.New("ecdh: invalid key")

// PublicKey is a ECDH public key.
type PublicKey struct {
	pubBytes  [GroupElementLength]byte
	b64String string
}

// Bytes returns the raw public key.
func (k *PublicKey) Bytes() []byte {
	return k.pubBytes[:]
}

// FromBytes deserializes the byte slice b into the PublicKey.
func (k *PublicKey) FromBytes(b []byte) error {
	if len(b) != PublicKeySize {
		return errInvalidKey
	}

	copy(k.pubBytes[:], b)
	k.rebuildB64String()

	return nil
}

// MarshalBinary is an implementation of a method on the
// BinaryMarshaler interface defined in https://golang.org/pkg/encoding/
func (k *PublicKey) MarshalBinary() ([]byte, error) {
	return k.Bytes(), nil
}

// UnmarshalBinary is an implementation of a method on the
// BinaryUnmarshaler interface defined in https://golang.org/pkg/encoding/
func (k *PublicKey) UnmarshalBinary(data []byte) error {
	return k.FromBytes(data)
}

// MarshalText is an implementation of a method on the
// TextMarshaler interface defined in https://golang.org/pkg/encoding/
func (k *PublicKey) MarshalText() ([]byte, error) {
	return []byte(base64.StdEncoding.EncodeToString(k.Bytes())), nil
}

// UnmarshalText is an implementation of a method on the
// TextUnmarshaler interface defined in https://golang.org/pkg/encoding/
func (k *PublicKey) UnmarshalText(data []byte) error {
	raw, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return err
	}
	return k.FromBytes(raw)
}

// Reset clears the PublicKey structure such that no sensitive data is left
// in memory.
func (k *PublicKey) Reset() {
	utils.ExplicitBzero(k.pubBytes[:])
	k.b64String = "[scrubbed]"
}

// Blind blinds the public key with the provided blinding factor.
func (k *PublicKey) Blind(blindingFactor *[GroupElementLength]byte) {
	Exp(&k.pubBytes, &k.pubBytes, blindingFactor)
}

// String returns the public key as a base64 encoded string.
func (k *PublicKey) String() string {
	return k.b64String
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
	return fmt.Errorf("ecdh: key is neither Base16 nor Base64")
}

// ToPEMFile writes out the PublicKey to a PEM file at path f.
func (k *PublicKey) ToPEMFile(f string) error {
	const keyType = "X25519 PUBLIC KEY"

	if utils.CtIsZero(k.pubBytes[:]) {
		return fmt.Errorf("ecdh: attemted to serialize scrubbed key")
	}
	blk := &pem.Block{
		Type:  keyType,
		Bytes: k.Bytes(),
	}
	return ioutil.WriteFile(f, pem.EncodeToMemory(blk), 0600)
}

// FromPEMFile reads the PublicKey from a PEM file at path f.
func (k *PublicKey) FromPEMFile(f string) error {
	const keyType = "X25519 PUBLIC KEY"

	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}
	blk, _ := pem.Decode(buf)
	if blk == nil {
		return fmt.Errorf("ecdh: failed to decode PEM file %v", f)
	}
	if blk.Type != keyType {
		return fmt.Errorf("ecdh: attempted to decode PEM file with wrong key type")
	}
	return k.FromBytes(blk.Bytes)
}

func (k *PublicKey) rebuildB64String() {
	k.b64String = base64.StdEncoding.EncodeToString(k.Bytes())
}

// Equal returns true iff the public key is byte for byte identical.
func (k *PublicKey) Equal(cmp *PublicKey) bool {
	return subtle.ConstantTimeCompare(k.pubBytes[:], cmp.pubBytes[:]) == 1
}

// PrivateKey is a ECDH private key.
type PrivateKey struct {
	pubKey    PublicKey
	privBytes [GroupElementLength]byte
}

// Bytes returns the raw private key.
func (k *PrivateKey) Bytes() []byte {
	return k.privBytes[:]
}

// MarshalBinary is an implementation of a method on the
// BinaryMarshaler interface defined in https://golang.org/pkg/encoding/
func (k *PrivateKey) MarshalBinary() ([]byte, error) {
	return k.Bytes(), nil
}

// UnmarshalBinary is an implementation of a method on the
// BinaryUnmarshaler interface defined in https://golang.org/pkg/encoding/
func (k *PrivateKey) UnmarshalBinary(data []byte) error {
	return k.FromBytes(data)
}

// FromBytes deserializes the byte slice b into the PrivateKey.
func (k *PrivateKey) FromBytes(b []byte) error {
	if len(b) != PrivateKeySize {
		return errInvalidKey
	}

	copy(k.privBytes[:], b)
	expG(&k.pubKey.pubBytes, &k.privBytes)
	k.pubKey.rebuildB64String()

	return nil
}

// Exp calculates the shared secret with the provided public key.
func (k *PrivateKey) Exp(sharedSecret *[GroupElementLength]byte, publicKey *PublicKey) {
	Exp(sharedSecret, &publicKey.pubBytes, &k.privBytes)
}

// Reset clears the PrivateKey structure such that no sensitive data is left
// in memory.
func (k *PrivateKey) Reset() {
	k.pubKey.Reset()
	utils.ExplicitBzero(k.privBytes[:])
}

// PublicKey returns the PublicKey corresponding to the PrivateKey.
func (k *PrivateKey) PublicKey() *PublicKey {
	return &k.pubKey
}

// NewKeypair generates a new PrivateKey sampled from the provided entropy
// source.
func NewKeypair(r io.Reader) (*PrivateKey, error) {
	k := new(PrivateKey)
	if _, err := io.ReadFull(r, k.privBytes[:]); err != nil {
		return nil, err
	}

	expG(&k.pubKey.pubBytes, &k.privBytes)
	k.pubKey.rebuildB64String()

	return k, nil
}

// Load loads a new PrivateKey from the PEM encoded file privFile, optionally
// creating and saving a PrivateKey instead if an entropy source is provided.
// If pubFile is specified and a key has been created, the corresponding
// PublicKey will be wrtten to pubFile in PEM format.
func Load(privFile, pubFile string, r io.Reader) (*PrivateKey, error) {
	const keyType = "X25519 PRIVATE KEY"

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

// Exp sets the group element dst to be the result of x^y, over the ECDH
// group.
func Exp(dst, x, y *[GroupElementLength]byte) {
	curve25519.ScalarMult(dst, y, x)
}

func expG(dst, y *[GroupElementLength]byte) {
	curve25519.ScalarBaseMult(dst, y)
}
