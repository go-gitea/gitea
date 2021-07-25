// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"bytes"
	"crypto"
	"crypto/cipher"
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"strconv"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/internal/ecc"
	"golang.org/x/crypto/curve25519"

	"github.com/ProtonMail/go-crypto/openpgp/ecdh"
	"github.com/ProtonMail/go-crypto/openpgp/elgamal"
	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/encoding"
	"github.com/ProtonMail/go-crypto/openpgp/s2k"
	"golang.org/x/crypto/ed25519"
)

// PrivateKey represents a possibly encrypted private key. See RFC 4880,
// section 5.5.3.
type PrivateKey struct {
	PublicKey
	Encrypted     bool // if true then the private key is unavailable until Decrypt has been called.
	encryptedData []byte
	cipher        CipherFunction
	s2k           func(out, in []byte)
	// An *{rsa|dsa|elgamal|ecdh|ecdsa|ed25519}.PrivateKey or
	// crypto.Signer/crypto.Decrypter (Decryptor RSA only).
	PrivateKey   interface{}
	sha1Checksum bool
	iv           []byte

	// Type of encryption of the S2K packet
	// Allowed values are 0 (Not encrypted), 254 (SHA1), or
	// 255 (2-byte checksum)
	s2kType S2KType
	// Full parameters of the S2K packet
	s2kParams *s2k.Params
}

//S2KType s2k packet type
type S2KType uint8

const (
	// S2KNON unencrypt
	S2KNON S2KType = 0
	// S2KSHA1 sha1 sum check
	S2KSHA1 S2KType = 254
	// S2KCHECKSUM sum check
	S2KCHECKSUM S2KType = 255
)

func NewRSAPrivateKey(creationTime time.Time, priv *rsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewRSAPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewDSAPrivateKey(creationTime time.Time, priv *dsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewDSAPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewElGamalPrivateKey(creationTime time.Time, priv *elgamal.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewElGamalPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewECDSAPrivateKey(creationTime time.Time, priv *ecdsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewECDSAPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewEdDSAPrivateKey(creationTime time.Time, priv *ed25519.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pub := priv.Public().(ed25519.PublicKey)
	pk.PublicKey = *NewEdDSAPublicKey(creationTime, &pub)
	pk.PrivateKey = priv
	return pk
}

func NewECDHPrivateKey(creationTime time.Time, priv *ecdh.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewECDHPublicKey(creationTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

// NewSignerPrivateKey creates a PrivateKey from a crypto.Signer that
// implements RSA, ECDSA or EdDSA.
func NewSignerPrivateKey(creationTime time.Time, signer crypto.Signer) *PrivateKey {
	pk := new(PrivateKey)
	// In general, the public Keys should be used as pointers. We still
	// type-switch on the values, for backwards-compatibility.
	switch pubkey := signer.Public().(type) {
	case *rsa.PublicKey:
		pk.PublicKey = *NewRSAPublicKey(creationTime, pubkey)
	case rsa.PublicKey:
		pk.PublicKey = *NewRSAPublicKey(creationTime, &pubkey)
	case *ecdsa.PublicKey:
		pk.PublicKey = *NewECDSAPublicKey(creationTime, pubkey)
	case ecdsa.PublicKey:
		pk.PublicKey = *NewECDSAPublicKey(creationTime, &pubkey)
	case *ed25519.PublicKey:
		pk.PublicKey = *NewEdDSAPublicKey(creationTime, pubkey)
	case ed25519.PublicKey:
		pk.PublicKey = *NewEdDSAPublicKey(creationTime, &pubkey)
	default:
		panic("openpgp: unknown crypto.Signer type in NewSignerPrivateKey")
	}
	pk.PrivateKey = signer
	return pk
}

// NewDecrypterPrivateKey creates a PrivateKey from a *{rsa|elgamal|ecdh}.PrivateKey.
func NewDecrypterPrivateKey(creationTime time.Time, decrypter interface{}) *PrivateKey {
	pk := new(PrivateKey)
	switch priv := decrypter.(type) {
	case *rsa.PrivateKey:
		pk.PublicKey = *NewRSAPublicKey(creationTime, &priv.PublicKey)
	case *elgamal.PrivateKey:
		pk.PublicKey = *NewElGamalPublicKey(creationTime, &priv.PublicKey)
	case *ecdh.PrivateKey:
		pk.PublicKey = *NewECDHPublicKey(creationTime, &priv.PublicKey)
	default:
		panic("openpgp: unknown decrypter type in NewDecrypterPrivateKey")
	}
	pk.PrivateKey = decrypter
	return pk
}

func (pk *PrivateKey) parse(r io.Reader) (err error) {
	err = (&pk.PublicKey).parse(r)
	if err != nil {
		return
	}
	v5 := pk.PublicKey.Version == 5

	var buf [1]byte
	_, err = readFull(r, buf[:])
	if err != nil {
		return
	}
	pk.s2kType = S2KType(buf[0])
	var optCount [1]byte
	if v5 {
		if _, err = readFull(r, optCount[:]); err != nil {
			return
		}
	}

	switch pk.s2kType {
	case S2KNON:
		pk.s2k = nil
		pk.Encrypted = false
	case S2KSHA1, S2KCHECKSUM:
		if v5 && pk.s2kType == S2KCHECKSUM {
			return errors.StructuralError("wrong s2k identifier for version 5")
		}
		_, err = readFull(r, buf[:])
		if err != nil {
			return
		}
		pk.cipher = CipherFunction(buf[0])
		pk.s2kParams, err = s2k.ParseIntoParams(r)
		if err != nil {
			return
		}
		if pk.s2kParams.Dummy() {
			return
		}
		pk.s2k, err = pk.s2kParams.Function()
		if err != nil {
			return
		}
		pk.Encrypted = true
		if pk.s2kType == S2KSHA1 {
			pk.sha1Checksum = true
		}
	default:
		return errors.UnsupportedError("deprecated s2k function in private key")
	}

	if pk.Encrypted {
		blockSize := pk.cipher.blockSize()
		if blockSize == 0 {
			return errors.UnsupportedError("unsupported cipher in private key: " + strconv.Itoa(int(pk.cipher)))
		}
		pk.iv = make([]byte, blockSize)
		_, err = readFull(r, pk.iv)
		if err != nil {
			return
		}
	}

	var privateKeyData []byte
	if v5 {
		var n [4]byte /* secret material four octet count */
		_, err = readFull(r, n[:])
		if err != nil {
			return
		}
		count := uint32(uint32(n[0])<<24 | uint32(n[1])<<16 | uint32(n[2])<<8 | uint32(n[3]))
		if !pk.Encrypted {
			count = count + 2 /* two octet checksum */
		}
		privateKeyData = make([]byte, count)
		_, err = readFull(r, privateKeyData)
		if err != nil {
			return
		}
	} else {
		privateKeyData, err = ioutil.ReadAll(r)
		if err != nil {
			return
		}
	}
	if !pk.Encrypted {
		return pk.parsePrivateKey(privateKeyData)
	}

	pk.encryptedData = privateKeyData
	return
}

// Dummy returns true if the private key is a dummy key. This is a GNU extension.
func (pk *PrivateKey) Dummy() bool {
	return pk.s2kParams.Dummy()
}

func mod64kHash(d []byte) uint16 {
	var h uint16
	for _, b := range d {
		h += uint16(b)
	}
	return h
}

func (pk *PrivateKey) Serialize(w io.Writer) (err error) {
	contents := bytes.NewBuffer(nil)
	err = pk.PublicKey.serializeWithoutHeaders(contents)
	if err != nil {
		return
	}
	if _, err = contents.Write([]byte{uint8(pk.s2kType)}); err != nil {
		return
	}

	optional := bytes.NewBuffer(nil)
	if pk.Encrypted || pk.Dummy() {
		optional.Write([]byte{uint8(pk.cipher)})
		if err := pk.s2kParams.Serialize(optional); err != nil {
			return err
		}
		if pk.Encrypted {
			optional.Write(pk.iv)
		}
	}
	if pk.Version == 5 {
		contents.Write([]byte{uint8(optional.Len())})
	}
	io.Copy(contents, optional)

	if !pk.Dummy() {
		l := 0
		var priv []byte
		if !pk.Encrypted {
			buf := bytes.NewBuffer(nil)
			err = pk.serializePrivateKey(buf)
			if err != nil {
				return err
			}
			l = buf.Len()
			if pk.sha1Checksum {
				h := sha1.New()
				h.Write(buf.Bytes())
				buf.Write(h.Sum(nil))
			} else {
				checksum := mod64kHash(buf.Bytes())
				buf.Write([]byte{byte(checksum >> 8), byte(checksum)})
			}
			priv = buf.Bytes()
		} else {
			priv, l = pk.encryptedData, len(pk.encryptedData)
		}

		if pk.Version == 5 {
			contents.Write([]byte{byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)})
		}
		contents.Write(priv)
	}

	ptype := packetTypePrivateKey
	if pk.IsSubkey {
		ptype = packetTypePrivateSubkey
	}
	err = serializeHeader(w, ptype, contents.Len())
	if err != nil {
		return
	}
	_, err = io.Copy(w, contents)
	if err != nil {
		return
	}
	return
}

func serializeRSAPrivateKey(w io.Writer, priv *rsa.PrivateKey) error {
	if _, err := w.Write(new(encoding.MPI).SetBig(priv.D).EncodedBytes()); err != nil {
		return err
	}
	if _, err := w.Write(new(encoding.MPI).SetBig(priv.Primes[1]).EncodedBytes()); err != nil {
		return err
	}
	if _, err := w.Write(new(encoding.MPI).SetBig(priv.Primes[0]).EncodedBytes()); err != nil {
		return err
	}
	_, err := w.Write(new(encoding.MPI).SetBig(priv.Precomputed.Qinv).EncodedBytes())
	return err
}

func serializeDSAPrivateKey(w io.Writer, priv *dsa.PrivateKey) error {
	_, err := w.Write(new(encoding.MPI).SetBig(priv.X).EncodedBytes())
	return err
}

func serializeElGamalPrivateKey(w io.Writer, priv *elgamal.PrivateKey) error {
	_, err := w.Write(new(encoding.MPI).SetBig(priv.X).EncodedBytes())
	return err
}

func serializeECDSAPrivateKey(w io.Writer, priv *ecdsa.PrivateKey) error {
	_, err := w.Write(new(encoding.MPI).SetBig(priv.D).EncodedBytes())
	return err
}

func serializeEdDSAPrivateKey(w io.Writer, priv *ed25519.PrivateKey) error {
	keySize := ed25519.PrivateKeySize - ed25519.PublicKeySize
	_, err := w.Write(encoding.NewMPI((*priv)[:keySize]).EncodedBytes())
	return err
}

func serializeECDHPrivateKey(w io.Writer, priv *ecdh.PrivateKey) error {
	_, err := w.Write(encoding.NewMPI(priv.D).EncodedBytes())
	return err
}

// Decrypt decrypts an encrypted private key using a passphrase.
func (pk *PrivateKey) Decrypt(passphrase []byte) error {
	if pk.Dummy() {
		return errors.ErrDummyPrivateKey("dummy key found")
	}
	if !pk.Encrypted {
		return nil
	}

	key := make([]byte, pk.cipher.KeySize())
	pk.s2k(key, passphrase)
	block := pk.cipher.new(key)
	cfb := cipher.NewCFBDecrypter(block, pk.iv)

	data := make([]byte, len(pk.encryptedData))
	cfb.XORKeyStream(data, pk.encryptedData)

	if pk.sha1Checksum {
		if len(data) < sha1.Size {
			return errors.StructuralError("truncated private key data")
		}
		h := sha1.New()
		h.Write(data[:len(data)-sha1.Size])
		sum := h.Sum(nil)
		if !bytes.Equal(sum, data[len(data)-sha1.Size:]) {
			return errors.StructuralError("private key checksum failure")
		}
		data = data[:len(data)-sha1.Size]
	} else {
		if len(data) < 2 {
			return errors.StructuralError("truncated private key data")
		}
		var sum uint16
		for i := 0; i < len(data)-2; i++ {
			sum += uint16(data[i])
		}
		if data[len(data)-2] != uint8(sum>>8) ||
			data[len(data)-1] != uint8(sum) {
			return errors.StructuralError("private key checksum failure")
		}
		data = data[:len(data)-2]
	}

	err := pk.parsePrivateKey(data)
	if _, ok := err.(errors.KeyInvalidError); ok {
		return errors.KeyInvalidError("invalid key parameters")
	}
	if err != nil {
		return err
	}

	// Mark key as unencrypted
	pk.s2kType = S2KNON
	pk.s2k = nil
	pk.Encrypted = false
	pk.encryptedData = nil

	return nil
}

// Encrypt encrypts an unencrypted private key using a passphrase.
func (pk *PrivateKey) Encrypt(passphrase []byte) error {
	priv := bytes.NewBuffer(nil)
	err := pk.serializePrivateKey(priv)
	if err != nil {
		return err
	}

	//Default config of private key encryption
	pk.cipher = CipherAES256
	s2kConfig := &s2k.Config{
		S2KMode:  3, //Iterated
		S2KCount: 65536,
		Hash:     crypto.SHA256,
	}

	pk.s2kParams, err = s2k.Generate(rand.Reader, s2kConfig)
	if err != nil {
		return err
	}
	privateKeyBytes := priv.Bytes()
	key := make([]byte, pk.cipher.KeySize())

	pk.sha1Checksum = true
	pk.s2k, err = pk.s2kParams.Function()
	if err != nil {
		return err
	}
	pk.s2k(key, passphrase)
	block := pk.cipher.new(key)
	pk.iv = make([]byte, pk.cipher.blockSize())
	_, err = rand.Read(pk.iv)
	if err != nil {
		return err
	}
	cfb := cipher.NewCFBEncrypter(block, pk.iv)

	if pk.sha1Checksum {
		pk.s2kType = S2KSHA1
		h := sha1.New()
		h.Write(privateKeyBytes)
		sum := h.Sum(nil)
		privateKeyBytes = append(privateKeyBytes, sum...)
	} else {
		pk.s2kType = S2KCHECKSUM
		var sum uint16
		for _, b := range privateKeyBytes {
			sum += uint16(b)
		}
		priv.Write([]byte{uint8(sum >> 8), uint8(sum)})
	}

	pk.encryptedData = make([]byte, len(privateKeyBytes))
	cfb.XORKeyStream(pk.encryptedData, privateKeyBytes)
	pk.Encrypted = true
	pk.PrivateKey = nil
	return err
}

func (pk *PrivateKey) serializePrivateKey(w io.Writer) (err error) {
	switch priv := pk.PrivateKey.(type) {
	case *rsa.PrivateKey:
		err = serializeRSAPrivateKey(w, priv)
	case *dsa.PrivateKey:
		err = serializeDSAPrivateKey(w, priv)
	case *elgamal.PrivateKey:
		err = serializeElGamalPrivateKey(w, priv)
	case *ecdsa.PrivateKey:
		err = serializeECDSAPrivateKey(w, priv)
	case *ed25519.PrivateKey:
		err = serializeEdDSAPrivateKey(w, priv)
	case *ecdh.PrivateKey:
		err = serializeECDHPrivateKey(w, priv)
	default:
		err = errors.InvalidArgumentError("unknown private key type")
	}
	return
}

func (pk *PrivateKey) parsePrivateKey(data []byte) (err error) {
	switch pk.PublicKey.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly, PubKeyAlgoRSAEncryptOnly:
		return pk.parseRSAPrivateKey(data)
	case PubKeyAlgoDSA:
		return pk.parseDSAPrivateKey(data)
	case PubKeyAlgoElGamal:
		return pk.parseElGamalPrivateKey(data)
	case PubKeyAlgoECDSA:
		return pk.parseECDSAPrivateKey(data)
	case PubKeyAlgoECDH:
		return pk.parseECDHPrivateKey(data)
	case PubKeyAlgoEdDSA:
		return pk.parseEdDSAPrivateKey(data)
	}
	panic("impossible")
}

func (pk *PrivateKey) parseRSAPrivateKey(data []byte) (err error) {
	rsaPub := pk.PublicKey.PublicKey.(*rsa.PublicKey)
	rsaPriv := new(rsa.PrivateKey)
	rsaPriv.PublicKey = *rsaPub

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	p := new(encoding.MPI)
	if _, err := p.ReadFrom(buf); err != nil {
		return err
	}

	q := new(encoding.MPI)
	if _, err := q.ReadFrom(buf); err != nil {
		return err
	}

	rsaPriv.D = new(big.Int).SetBytes(d.Bytes())
	rsaPriv.Primes = make([]*big.Int, 2)
	rsaPriv.Primes[0] = new(big.Int).SetBytes(p.Bytes())
	rsaPriv.Primes[1] = new(big.Int).SetBytes(q.Bytes())
	if err := rsaPriv.Validate(); err != nil {
		return errors.KeyInvalidError(err.Error())
	}
	rsaPriv.Precompute()
	pk.PrivateKey = rsaPriv

	return nil
}

func (pk *PrivateKey) parseDSAPrivateKey(data []byte) (err error) {
	dsaPub := pk.PublicKey.PublicKey.(*dsa.PublicKey)
	dsaPriv := new(dsa.PrivateKey)
	dsaPriv.PublicKey = *dsaPub

	buf := bytes.NewBuffer(data)
	x := new(encoding.MPI)
	if _, err := x.ReadFrom(buf); err != nil {
		return err
	}

	dsaPriv.X = new(big.Int).SetBytes(x.Bytes())
	if err := validateDSAParameters(dsaPriv); err != nil {
		return err
	}
	pk.PrivateKey = dsaPriv

	return nil
}

func (pk *PrivateKey) parseElGamalPrivateKey(data []byte) (err error) {
	pub := pk.PublicKey.PublicKey.(*elgamal.PublicKey)
	priv := new(elgamal.PrivateKey)
	priv.PublicKey = *pub

	buf := bytes.NewBuffer(data)
	x := new(encoding.MPI)
	if _, err := x.ReadFrom(buf); err != nil {
		return err
	}

	priv.X = new(big.Int).SetBytes(x.Bytes())
	if err := validateElGamalParameters(priv); err != nil {
		return err
	}
	pk.PrivateKey = priv

	return nil
}

func (pk *PrivateKey) parseECDSAPrivateKey(data []byte) (err error) {
	ecdsaPub := pk.PublicKey.PublicKey.(*ecdsa.PublicKey)
	ecdsaPriv := new(ecdsa.PrivateKey)
	ecdsaPriv.PublicKey = *ecdsaPub

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	ecdsaPriv.D = new(big.Int).SetBytes(d.Bytes())
	if err := validateECDSAParameters(ecdsaPriv); err != nil {
		return err
	}
	pk.PrivateKey = ecdsaPriv

	return nil
}

func (pk *PrivateKey) parseECDHPrivateKey(data []byte) (err error) {
	ecdhPub := pk.PublicKey.PublicKey.(*ecdh.PublicKey)
	ecdhPriv := new(ecdh.PrivateKey)
	ecdhPriv.PublicKey = *ecdhPub

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	ecdhPriv.D = d.Bytes()
	if err := validateECDHParameters(ecdhPriv); err != nil {
		return err
	}
	pk.PrivateKey = ecdhPriv

	return nil
}

func (pk *PrivateKey) parseEdDSAPrivateKey(data []byte) (err error) {
	eddsaPub := pk.PublicKey.PublicKey.(*ed25519.PublicKey)
	eddsaPriv := make(ed25519.PrivateKey, ed25519.PrivateKeySize)

	buf := bytes.NewBuffer(data)
	d := new(encoding.MPI)
	if _, err := d.ReadFrom(buf); err != nil {
		return err
	}

	priv := d.Bytes()
	copy(eddsaPriv[32-len(priv):32], priv)
	copy(eddsaPriv[32:], (*eddsaPub)[:])
	if err := validateEdDSAParameters(&eddsaPriv); err != nil {
		return err
	}
	pk.PrivateKey = &eddsaPriv

	return nil
}

func validateECDSAParameters(priv *ecdsa.PrivateKey) error {
	return validateCommonECC(priv.Curve, priv.D.Bytes(), priv.X, priv.Y)
}

func validateECDHParameters(priv *ecdh.PrivateKey) error {
	if priv.CurveType != ecc.Curve25519 {
		return validateCommonECC(priv.Curve, priv.D, priv.X, priv.Y)
	}
	// Handle Curve25519
	Q := priv.X.Bytes()[1:]
	var d [32]byte
	// Copy reversed d
	l := len(priv.D)
	for i := 0; i < l; i++ {
		d[i] = priv.D[l-i-1]
	}
	var expectedQ [32]byte
	curve25519.ScalarBaseMult(&expectedQ, &d)
	if !bytes.Equal(Q, expectedQ[:]) {
		return errors.KeyInvalidError("ECDH curve25519: invalid point")
	}
	return nil
}

func validateCommonECC(curve elliptic.Curve, d []byte, X, Y *big.Int) error {
	// the public point should not be at infinity (0,0)
	zero := new(big.Int)
	if X.Cmp(zero) == 0 && Y.Cmp(zero) == 0 {
		return errors.KeyInvalidError(fmt.Sprintf("ecc (%s): infinity point", curve.Params().Name))
	}
	// re-derive the public point Q' = (X,Y) = dG
	// to compare to declared Q in public key
	expectedX, expectedY := curve.ScalarBaseMult(d)
	if X.Cmp(expectedX) != 0 || Y.Cmp(expectedY) != 0 {
		return errors.KeyInvalidError(fmt.Sprintf("ecc (%s): invalid point", curve.Params().Name))
	}
	return nil
}

func validateEdDSAParameters(priv *ed25519.PrivateKey) error {
	// In EdDSA, the serialized public point is stored as part of private key (together with the seed),
	// hence we can re-derive the key from the seed
	seed := priv.Seed()
	expectedPriv := ed25519.NewKeyFromSeed(seed)
	if !bytes.Equal(*priv, expectedPriv) {
		return errors.KeyInvalidError("eddsa: invalid point")
	}
	return nil
}

func validateDSAParameters(priv *dsa.PrivateKey) error {
	p := priv.P // group prime
	q := priv.Q // subgroup order
	g := priv.G // g has order q mod p
	x := priv.X // secret
	y := priv.Y // y == g**x mod p
	one := big.NewInt(1)
	// expect g, y >= 2 and g < p
	if g.Cmp(one) <= 0 || y.Cmp(one) <= 0 || g.Cmp(p) > 0 {
		return errors.KeyInvalidError("dsa: invalid group")
	}
	// expect p > q
	if p.Cmp(q) <= 0 {
		return errors.KeyInvalidError("dsa: invalid group prime")
	}
	// q should be large enough and divide p-1
	pSub1 := new(big.Int).Sub(p, one)
	if q.BitLen() < 150 || new(big.Int).Mod(pSub1, q).Cmp(big.NewInt(0)) != 0 {
		return errors.KeyInvalidError("dsa: invalid order")
	}
	// confirm that g has order q mod p
	if !q.ProbablyPrime(32) || new(big.Int).Exp(g, q, p).Cmp(one) != 0 {
		return errors.KeyInvalidError("dsa: invalid order")
	}
	// check y
	if new(big.Int).Exp(g, x, p).Cmp(y) != 0 {
		return errors.KeyInvalidError("dsa: mismatching values")
	}

	return nil
}

func validateElGamalParameters(priv *elgamal.PrivateKey) error {
	p := priv.P // group prime
	g := priv.G // g has order p-1 mod p
	x := priv.X // secret
	y := priv.Y // y == g**x mod p
	one := big.NewInt(1)
	// Expect g, y >= 2 and g < p
	if g.Cmp(one) <= 0 || y.Cmp(one) <= 0 || g.Cmp(p) > 0 {
		return errors.KeyInvalidError("elgamal: invalid group")
	}
	if p.BitLen() < 1024 {
		return errors.KeyInvalidError("elgamal: group order too small")
	}
	pSub1 := new(big.Int).Sub(p, one)
	if new(big.Int).Exp(g, pSub1, p).Cmp(one) != 0 {
		return errors.KeyInvalidError("elgamal: invalid group")
	}
	// Since p-1 is not prime, g might have a smaller order that divides p-1.
	// We cannot confirm the exact order of g, but we make sure it is not too small.
	gExpI := new(big.Int).Set(g)
	i := 1
	threshold := 2 << 17 // we want order > threshold
	for i < threshold {
		i++ // we check every order to make sure key validation is not easily bypassed by guessing y'
		gExpI.Mod(new(big.Int).Mul(gExpI, g), p)
		if gExpI.Cmp(one) == 0 {
			return errors.KeyInvalidError("elgamal: order too small")
		}
	}
	// Check y
	if new(big.Int).Exp(g, x, p).Cmp(y) != 0 {
		return errors.KeyInvalidError("elgamal: mismatching values")
	}

	return nil
}
