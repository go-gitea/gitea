// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ecdh implements ECDH encryption, suitable for OpenPGP,
// as specified in RFC 6637, section 8.
package ecdh

import (
	"errors"
	"io"
	"math/big"

	"github.com/ProtonMail/go-crypto/openpgp/aes/keywrap"
	"github.com/ProtonMail/go-crypto/openpgp/internal/ecc"
	"golang.org/x/crypto/curve25519"
)

// Generates a private-public key-pair.
// 'priv' is a private key; a scalar belonging to the set
// 2^{254} + 8 * [0, 2^{251}), in order to avoid the small subgroup of the
// curve. 'pub' is simply 'priv' * G where G is the base point.
// See https://cr.yp.to/ecdh.html and RFC7748, sec 5.
func x25519GenerateKeyPairBytes(rand io.Reader) (priv [32]byte, pub [32]byte, err error) {
	var n, helper = new(big.Int), new(big.Int)
	n.SetUint64(1)
	n.Lsh(n, 252)
	helper.SetString("27742317777372353535851937790883648493", 10)
	n.Add(n, helper)

	for true {
		_, err = io.ReadFull(rand, priv[:])
		if err != nil {
			return
		}
		// The following ensures that the private key is a number of the form
		// 2^{254} + 8 * [0, 2^{251}), in order to avoid the small subgroup of
		// of the curve.
		priv[0] &= 248
		priv[31] &= 127
		priv[31] |= 64

		// If the scalar is out of range, sample another random number.
		if new(big.Int).SetBytes(priv[:]).Cmp(n) >= 0 {
			continue
		}

		curve25519.ScalarBaseMult(&pub, &priv)
		return
	}
	return
}

// X25519GenerateKey samples the key pair according to the correct distribution.
// It also sets the given key-derivation function and returns the *PrivateKey
// object along with an error.
func X25519GenerateKey(rand io.Reader, kdf KDF) (priv *PrivateKey, err error) {
	ci := ecc.FindByName("Curve25519")
	priv = new(PrivateKey)
	priv.PublicKey.Curve = ci.Curve
	d, pubKey, err := x25519GenerateKeyPairBytes(rand)
	if err != nil {
		return nil, err
	}
	priv.PublicKey.KDF = kdf
	priv.D = make([]byte, 32)
	copyReversed(priv.D, d[:])
	priv.PublicKey.CurveType = ci.CurveType
	priv.PublicKey.Curve = ci.Curve
	/*
	 * Note that ECPoint.point differs from the definition of public keys in
	 * [Curve25519] in two ways: (1) the byte-ordering is big-endian, which is
	 * more uniform with how big integers are represented in TLS, and (2) there
	 * is an additional length byte (so ECpoint.point is actually 33 bytes),
	 * again for uniformity (and extensibility).
	 */
	var encodedKey = make([]byte, 33)
	encodedKey[0] = 0x40
	copy(encodedKey[1:], pubKey[:])
	priv.PublicKey.X = new(big.Int).SetBytes(encodedKey[:])
	priv.PublicKey.Y = new(big.Int)
	return priv, nil
}

func X25519Encrypt(random io.Reader, pub *PublicKey, msg, curveOID, fingerprint []byte) (vsG, c []byte, err error) {
	d, ephemeralKey, err := x25519GenerateKeyPairBytes(random)
	if err != nil {
		return nil, nil, err
	}
	var pubKey [32]byte

	if pub.X.BitLen() > 33*264 {
		return nil, nil, errors.New("ecdh: invalid key")
	}
	copy(pubKey[:], pub.X.Bytes()[1:])

	var zb [32]byte
	curve25519.ScalarBaseMult(&zb, &d)
	curve25519.ScalarMult(&zb, &d, &pubKey)
	z, err := buildKey(pub, zb[:], curveOID, fingerprint, false, false)

	if err != nil {
		return nil, nil, err
	}

	if c, err = keywrap.Wrap(z, msg); err != nil {
		return nil, nil, err
	}

	var vsg [33]byte
	vsg[0] = 0x40
	copy(vsg[1:], ephemeralKey[:])

	return vsg[:], c, nil
}

func X25519Decrypt(priv *PrivateKey, vsG, m, curveOID, fingerprint []byte) (msg []byte, err error) {
	var zb, d, ephemeralKey [32]byte
	if len(vsG) != 33 || vsG[0] != 0x40 {
		return nil, errors.New("ecdh: invalid key")
	}
	copy(ephemeralKey[:], vsG[1:33])

	copyReversed(d[:], priv.D)
	curve25519.ScalarBaseMult(&zb, &d)
	curve25519.ScalarMult(&zb, &d, &ephemeralKey)

	var c []byte

	for i := 0; i < 3; i++ {
		// Try buildKey three times for compat, see comments in buildKey.
		z, err := buildKey(&priv.PublicKey, zb[:], curveOID, fingerprint, i == 1, i == 2)
		if err != nil {
			return nil, err
		}

		res, err := keywrap.Unwrap(z, m)
		if i == 2 && err != nil {
			// Only return an error after we've tried all variants of buildKey.
			return nil, err
		}

		c = res
		if err == nil {
			break
		}
	}

	return c[:len(c)-int(c[len(c)-1])], nil
}

func copyReversed(out []byte, in []byte) {
	l := len(in)
	for i := 0; i < l; i++ {
		out[i] = in[l-i-1]
	}
}
