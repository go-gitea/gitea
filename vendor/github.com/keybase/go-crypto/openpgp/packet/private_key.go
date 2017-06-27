// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"bytes"
	"crypto/cipher"
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"strconv"
	"time"

	"github.com/keybase/go-crypto/ed25519"
	"github.com/keybase/go-crypto/openpgp/ecdh"
	"github.com/keybase/go-crypto/openpgp/elgamal"
	"github.com/keybase/go-crypto/openpgp/errors"
	"github.com/keybase/go-crypto/openpgp/s2k"
	"github.com/keybase/go-crypto/rsa"
)

// PrivateKey represents a possibly encrypted private key. See RFC 4880,
// section 5.5.3.
type PrivateKey struct {
	PublicKey
	Encrypted     bool // if true then the private key is unavailable until Decrypt has been called.
	encryptedData []byte
	cipher        CipherFunction
	s2k           func(out, in []byte)
	PrivateKey    interface{} // An *rsa.PrivateKey or *dsa.PrivateKey.
	sha1Checksum  bool
	iv            []byte
	s2kHeader     []byte
}

type EdDSAPrivateKey struct {
	PrivateKey
	seed parsedMPI
}

func (e *EdDSAPrivateKey) Sign(digest []byte) (R, S []byte, err error) {
	r := bytes.NewReader(e.seed.bytes)
	publicKey, privateKey, err := ed25519.GenerateKey(r)
	if err != nil {
		return nil, nil, err
	}

	if !bytes.Equal(publicKey, e.PublicKey.edk.p.bytes[1:]) { // [1:] because [0] is 0x40 mpi header
		return nil, nil, errors.UnsupportedError("EdDSA: Private key does not match public key.")
	}

	sig := ed25519.Sign(privateKey, digest)

	sigLen := ed25519.SignatureSize / 2
	return sig[:sigLen], sig[sigLen:], nil
}

func NewRSAPrivateKey(currentTime time.Time, priv *rsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewRSAPublicKey(currentTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewDSAPrivateKey(currentTime time.Time, priv *dsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewDSAPublicKey(currentTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewElGamalPrivateKey(currentTime time.Time, priv *elgamal.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewElGamalPublicKey(currentTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func NewECDSAPrivateKey(currentTime time.Time, priv *ecdsa.PrivateKey) *PrivateKey {
	pk := new(PrivateKey)
	pk.PublicKey = *NewECDSAPublicKey(currentTime, &priv.PublicKey)
	pk.PrivateKey = priv
	return pk
}

func (pk *PrivateKey) parse(r io.Reader) (err error) {
	err = (&pk.PublicKey).parse(r)
	if err != nil {
		return
	}
	var buf [1]byte
	_, err = readFull(r, buf[:])
	if err != nil {
		return
	}

	s2kType := buf[0]

	switch s2kType {
	case 0:
		pk.s2k = nil
		pk.Encrypted = false
	case 254, 255:
		_, err = readFull(r, buf[:])
		if err != nil {
			return
		}
		pk.cipher = CipherFunction(buf[0])
		pk.Encrypted = true
		pk.s2k, err = s2k.Parse(r)
		if err != nil {
			return
		}
		if s2kType == 254 {
			pk.sha1Checksum = true
		}
		// S2K == nil implies that we got a "GNU Dummy" S2K. For instance,
		// because our master secret key is on a USB key in a vault somewhere.
		// In that case, there is no further data to consume here.
		if pk.s2k == nil {
			pk.Encrypted = false
			return
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

	pk.encryptedData, err = ioutil.ReadAll(r)
	if err != nil {
		return
	}

	if !pk.Encrypted {
		return pk.parsePrivateKey(pk.encryptedData)
	}

	return
}

func mod64kHash(d []byte) uint16 {
	var h uint16
	for _, b := range d {
		h += uint16(b)
	}
	return h
}

// Encrypt is the counterpart to the Decrypt() method below. It encrypts
// the private key with the provided passphrase. If config is nil, then
// the standard, and sensible, defaults apply.
//
// A key will be derived from the given passphrase using S2K Specifier
// Type 3 (Iterated + Salted, see RFC-4880 Sec. 3.7.1.3). This choice
// is hardcoded in s2k.Serialize(). S2KCount is hardcoded to 0, which is
// equivalent to 65536. And the hash algorithm for key-derivation can be
// set with config. The encrypted PrivateKey, using the algorithm specified
// in config (if provided), is written out to the encryptedData member.
// When Serialize() is called, this encryptedData member will be
// serialized, using S2K Usage value of 254, and thus SHA1 checksum.
func (pk *PrivateKey) Encrypt(passphrase []byte, config *Config) (err error) {
	if pk.PrivateKey == nil {
		return errors.InvalidArgumentError("there is no private key to encrypt")
	}

	pk.sha1Checksum = true
	pk.cipher = config.Cipher()
	s2kConfig := s2k.Config{
		Hash:     config.Hash(),
		S2KCount: 0,
	}
	s2kBuf := bytes.NewBuffer(nil)
	derivedKey := make([]byte, pk.cipher.KeySize())
	err = s2k.Serialize(s2kBuf, derivedKey, config.Random(), passphrase, &s2kConfig)
	if err != nil {
		return err
	}

	pk.s2kHeader = s2kBuf.Bytes()
	// No good way to set pk.s2k but to call s2k.Parse(),
	// even though we have all the information here, but
	// most of the functions needed are private to s2k.
	pk.s2k, err = s2k.Parse(s2kBuf)
	pk.iv = make([]byte, pk.cipher.blockSize())
	if _, err = config.Random().Read(pk.iv); err != nil {
		return err
	}

	privateKeyBuf := bytes.NewBuffer(nil)
	if err = pk.serializePrivateKey(privateKeyBuf); err != nil {
		return err
	}

	checksum := sha1.Sum(privateKeyBuf.Bytes())
	if _, err = privateKeyBuf.Write(checksum[:]); err != nil {
		return err
	}

	pkData := privateKeyBuf.Bytes()
	block := pk.cipher.new(derivedKey)
	pk.encryptedData = make([]byte, len(pkData))
	cfb := cipher.NewCFBEncrypter(block, pk.iv)
	cfb.XORKeyStream(pk.encryptedData, pkData)
	pk.Encrypted = true
	return nil
}

func (pk *PrivateKey) Serialize(w io.Writer) (err error) {
	buf := bytes.NewBuffer(nil)
	err = pk.PublicKey.serializeWithoutHeaders(buf)
	if err != nil {
		return
	}

	privateKeyBuf := bytes.NewBuffer(nil)

	if pk.PrivateKey == nil {
		_, err = buf.Write([]byte{
			254,           // SHA-1 Convention
			9,             // Encryption scheme (AES256)
			101,           // GNU Extensions
			2,             // Hash value (SHA1)
			'G', 'N', 'U', // "GNU" as a string
			1, // Extension type 1001 (minus 1000)
		})
	} else if pk.Encrypted {
		_, err = buf.Write([]byte{
			254,             // SHA-1 Convention
			byte(pk.cipher), // Encryption scheme
		})
		if err != nil {
			return err
		}
		if _, err = buf.Write(pk.s2kHeader); err != nil {
			return err
		}
		if _, err = buf.Write(pk.iv); err != nil {
			return err
		}
		if _, err = privateKeyBuf.Write(pk.encryptedData); err != nil {
			return err
		}
	} else {
		buf.WriteByte(0 /* no encryption */)
		if err = pk.serializePrivateKey(privateKeyBuf); err != nil {
			return err
		}
	}

	ptype := packetTypePrivateKey
	contents := buf.Bytes()
	privateKeyBytes := privateKeyBuf.Bytes()
	if pk.IsSubkey {
		ptype = packetTypePrivateSubkey
	}
	totalLen := len(contents) + len(privateKeyBytes)
	if !pk.Encrypted {
		totalLen += 2
	}
	err = serializeHeader(w, ptype, totalLen)
	if err != nil {
		return
	}
	_, err = w.Write(contents)
	if err != nil {
		return
	}
	_, err = w.Write(privateKeyBytes)
	if err != nil {
		return
	}

	if len(privateKeyBytes) > 0 && !pk.Encrypted {
		checksum := mod64kHash(privateKeyBytes)
		var checksumBytes [2]byte
		checksumBytes[0] = byte(checksum >> 8)
		checksumBytes[1] = byte(checksum)
		_, err = w.Write(checksumBytes[:])
	}

	return
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
	case *ecdh.PrivateKey:
		err = serializeECDHPrivateKey(w, priv)
	case *EdDSAPrivateKey:
		err = serializeEdDSAPrivateKey(w, priv)
	default:
		err = errors.InvalidArgumentError("unknown private key type")
	}

	return err
}

func serializeRSAPrivateKey(w io.Writer, priv *rsa.PrivateKey) error {
	err := writeBig(w, priv.D)
	if err != nil {
		return err
	}
	err = writeBig(w, priv.Primes[1])
	if err != nil {
		return err
	}
	err = writeBig(w, priv.Primes[0])
	if err != nil {
		return err
	}
	return writeBig(w, priv.Precomputed.Qinv)
}

func serializeDSAPrivateKey(w io.Writer, priv *dsa.PrivateKey) error {
	return writeBig(w, priv.X)
}

func serializeElGamalPrivateKey(w io.Writer, priv *elgamal.PrivateKey) error {
	return writeBig(w, priv.X)
}

func serializeECDSAPrivateKey(w io.Writer, priv *ecdsa.PrivateKey) error {
	return writeBig(w, priv.D)
}

func serializeECDHPrivateKey(w io.Writer, priv *ecdh.PrivateKey) error {
	return writeBig(w, priv.X)
}

func serializeEdDSAPrivateKey(w io.Writer, priv *EdDSAPrivateKey) error {
	return writeMPI(w, priv.seed.bitLength, priv.seed.bytes)
}

// Decrypt decrypts an encrypted private key using a passphrase.
func (pk *PrivateKey) Decrypt(passphrase []byte) error {
	if !pk.Encrypted {
		return nil
	}
	// For GNU Dummy S2K, there's no key here, so don't do anything.
	if pk.s2k == nil {
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

	return pk.parsePrivateKey(data)
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
	d, _, err := readMPI(buf)
	if err != nil {
		return
	}
	p, _, err := readMPI(buf)
	if err != nil {
		return
	}
	q, _, err := readMPI(buf)
	if err != nil {
		return
	}

	rsaPriv.D = new(big.Int).SetBytes(d)
	rsaPriv.Primes = make([]*big.Int, 2)
	rsaPriv.Primes[0] = new(big.Int).SetBytes(p)
	rsaPriv.Primes[1] = new(big.Int).SetBytes(q)
	if err := rsaPriv.Validate(); err != nil {
		return err
	}
	rsaPriv.Precompute()
	pk.PrivateKey = rsaPriv
	pk.Encrypted = false
	pk.encryptedData = nil

	return nil
}

func (pk *PrivateKey) parseDSAPrivateKey(data []byte) (err error) {
	dsaPub := pk.PublicKey.PublicKey.(*dsa.PublicKey)
	dsaPriv := new(dsa.PrivateKey)
	dsaPriv.PublicKey = *dsaPub

	buf := bytes.NewBuffer(data)
	x, _, err := readMPI(buf)
	if err != nil {
		return
	}

	dsaPriv.X = new(big.Int).SetBytes(x)
	pk.PrivateKey = dsaPriv
	pk.Encrypted = false
	pk.encryptedData = nil

	return nil
}

func (pk *PrivateKey) parseElGamalPrivateKey(data []byte) (err error) {
	pub := pk.PublicKey.PublicKey.(*elgamal.PublicKey)
	priv := new(elgamal.PrivateKey)
	priv.PublicKey = *pub

	buf := bytes.NewBuffer(data)
	x, _, err := readMPI(buf)
	if err != nil {
		return
	}

	priv.X = new(big.Int).SetBytes(x)
	pk.PrivateKey = priv
	pk.Encrypted = false
	pk.encryptedData = nil

	return nil
}

func (pk *PrivateKey) parseECDHPrivateKey(data []byte) (err error) {
	pub := pk.PublicKey.PublicKey.(*ecdh.PublicKey)
	priv := new(ecdh.PrivateKey)
	priv.PublicKey = *pub

	buf := bytes.NewBuffer(data)
	d, _, err := readMPI(buf)
	if err != nil {
		return
	}

	priv.X = new(big.Int).SetBytes(d)
	pk.PrivateKey = priv
	pk.Encrypted = false
	pk.encryptedData = nil
	return nil
}

func (pk *PrivateKey) parseECDSAPrivateKey(data []byte) (err error) {
	ecdsaPub := pk.PublicKey.PublicKey.(*ecdsa.PublicKey)
	ecdsaPriv := new(ecdsa.PrivateKey)
	ecdsaPriv.PublicKey = *ecdsaPub

	buf := bytes.NewBuffer(data)
	d, _, err := readMPI(buf)
	if err != nil {
		return
	}

	ecdsaPriv.D = new(big.Int).SetBytes(d)
	pk.PrivateKey = ecdsaPriv
	pk.Encrypted = false
	pk.encryptedData = nil

	return nil
}

func (pk *PrivateKey) parseEdDSAPrivateKey(data []byte) (err error) {
	eddsaPriv := new(EdDSAPrivateKey)
	eddsaPriv.PublicKey = pk.PublicKey

	buf := bytes.NewBuffer(data)
	eddsaPriv.seed.bytes, eddsaPriv.seed.bitLength, err = readMPI(buf)
	if err != nil {
		return err
	}

	if bLen := len(eddsaPriv.seed.bytes); bLen != 32 { // 32 bytes private part of ed25519 key.
		return errors.UnsupportedError(fmt.Sprintf("Unexpected EdDSA private key length: %d", bLen))
	}

	pk.PrivateKey = eddsaPriv
	pk.Encrypted = false
	pk.encryptedData = nil

	return nil
}
