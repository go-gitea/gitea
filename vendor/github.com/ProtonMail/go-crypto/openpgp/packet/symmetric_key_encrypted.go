// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"bytes"
	"crypto/cipher"
	"io"
	"strconv"

	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/s2k"
)

// This is the largest session key that we'll support. Since no 512-bit cipher
// has even been seriously used, this is comfortably large.
const maxSessionKeySizeInBytes = 64

// SymmetricKeyEncrypted represents a passphrase protected session key. See RFC
// 4880, section 5.3.
type SymmetricKeyEncrypted struct {
	Version      int
	CipherFunc   CipherFunction
	Mode         AEADMode
	s2k          func(out, in []byte)
	aeadNonce    []byte
	encryptedKey []byte
}

func (ske *SymmetricKeyEncrypted) parse(r io.Reader) error {
	// RFC 4880, section 5.3.
	var buf [2]byte
	if _, err := readFull(r, buf[:]); err != nil {
		return err
	}
	ske.Version = int(buf[0])
	if ske.Version != 4 && ske.Version != 5 {
		return errors.UnsupportedError("unknown SymmetricKeyEncrypted version")
	}
	ske.CipherFunc = CipherFunction(buf[1])
	if ske.CipherFunc.KeySize() == 0 {
		return errors.UnsupportedError("unknown cipher: " + strconv.Itoa(int(buf[1])))
	}

	if ske.Version == 5 {
		mode := make([]byte, 1)
		if _, err := r.Read(mode); err != nil {
			return errors.StructuralError("cannot read AEAD octect from packet")
		}
		ske.Mode = AEADMode(mode[0])
	}

	var err error
	if ske.s2k, err = s2k.Parse(r); err != nil {
		if _, ok := err.(errors.ErrDummyPrivateKey); ok {
			return errors.UnsupportedError("missing key GNU extension in session key")
		}
		return err
	}

	if ske.Version == 5 {
		// AEAD nonce
		nonce := make([]byte, ske.Mode.NonceLength())
		_, err := readFull(r, nonce)
		if err != nil && err != io.ErrUnexpectedEOF {
			return err
		}
		ske.aeadNonce = nonce
	}

	encryptedKey := make([]byte, maxSessionKeySizeInBytes)
	// The session key may follow. We just have to try and read to find
	// out. If it exists then we limit it to maxSessionKeySizeInBytes.
	n, err := readFull(r, encryptedKey)
	if err != nil && err != io.ErrUnexpectedEOF {
		return err
	}

	if n != 0 {
		if n == maxSessionKeySizeInBytes {
			return errors.UnsupportedError("oversized encrypted session key")
		}
		ske.encryptedKey = encryptedKey[:n]
	}
	return nil
}

// Decrypt attempts to decrypt an encrypted session key and returns the key and
// the cipher to use when decrypting a subsequent Symmetrically Encrypted Data
// packet.
func (ske *SymmetricKeyEncrypted) Decrypt(passphrase []byte) ([]byte, CipherFunction, error) {
	key := make([]byte, ske.CipherFunc.KeySize())
	ske.s2k(key, passphrase)
	if len(ske.encryptedKey) == 0 {
		return key, ske.CipherFunc, nil
	}
	switch ske.Version {
	case 4:
		plaintextKey, cipherFunc, err := ske.decryptV4(key)
		return plaintextKey, cipherFunc, err
	case 5:
		plaintextKey, err := ske.decryptV5(key)
		return plaintextKey, CipherFunction(0), err
	}
	err := errors.UnsupportedError("unknown SymmetricKeyEncrypted version")
	return nil, CipherFunction(0), err
}

func (ske *SymmetricKeyEncrypted) decryptV4(key []byte) ([]byte, CipherFunction, error) {
	// the IV is all zeros
	iv := make([]byte, ske.CipherFunc.blockSize())
	c := cipher.NewCFBDecrypter(ske.CipherFunc.new(key), iv)
	plaintextKey := make([]byte, len(ske.encryptedKey))
	c.XORKeyStream(plaintextKey, ske.encryptedKey)
	cipherFunc := CipherFunction(plaintextKey[0])
	if cipherFunc.blockSize() == 0 {
		return nil, ske.CipherFunc, errors.UnsupportedError(
			"unknown cipher: " + strconv.Itoa(int(cipherFunc)))
	}
	plaintextKey = plaintextKey[1:]
	if len(plaintextKey) != cipherFunc.KeySize() {
		return nil, cipherFunc, errors.StructuralError(
			"length of decrypted key not equal to cipher keysize")
	}
	return plaintextKey, cipherFunc, nil
}

func (ske *SymmetricKeyEncrypted) decryptV5(key []byte) ([]byte, error) {
	blockCipher := CipherFunction(ske.CipherFunc).new(key)
	aead := ske.Mode.new(blockCipher)

	adata := []byte{0xc3, byte(5), byte(ske.CipherFunc), byte(ske.Mode)}
	plaintextKey, err := aead.Open(nil, ske.aeadNonce, ske.encryptedKey, adata)
	if err != nil {
		return nil, err
	}
	return plaintextKey, nil
}

// SerializeSymmetricKeyEncrypted serializes a symmetric key packet to w.
// The packet contains a random session key, encrypted by a key derived from
// the given passphrase. The session key is returned and must be passed to
// SerializeSymmetricallyEncrypted or SerializeAEADEncrypted, depending on
// whether config.AEADConfig != nil.
// If config is nil, sensible defaults will be used.
func SerializeSymmetricKeyEncrypted(w io.Writer, passphrase []byte, config *Config) (key []byte, err error) {
	cipherFunc := config.Cipher()
	keySize := cipherFunc.KeySize()
	if keySize == 0 {
		return nil, errors.UnsupportedError("unknown cipher: " + strconv.Itoa(int(cipherFunc)))
	}

	sessionKey := make([]byte, keySize)
	_, err = io.ReadFull(config.Random(), sessionKey)
	if err != nil {
		return
	}

	err = SerializeSymmetricKeyEncryptedReuseKey(w, sessionKey, passphrase, config)
	if err != nil {
		return
	}

	key = sessionKey
	return
}

// SerializeSymmetricKeyEncryptedReuseKey serializes a symmetric key packet to w.
// The packet contains the given session key, encrypted by a key derived from
// the given passphrase. The session key must be passed to
// SerializeSymmetricallyEncrypted or SerializeAEADEncrypted, depending on
// whether config.AEADConfig != nil.
// If config is nil, sensible defaults will be used.
func SerializeSymmetricKeyEncryptedReuseKey(w io.Writer, sessionKey []byte, passphrase []byte, config *Config) (err error) {
	var version int
	if config.AEAD() != nil {
		version = 5
	} else {
		version = 4
	}
	cipherFunc := config.Cipher()
	keySize := cipherFunc.KeySize()
	if keySize == 0 {
		return errors.UnsupportedError("unknown cipher: " + strconv.Itoa(int(cipherFunc)))
	}

	s2kBuf := new(bytes.Buffer)
	keyEncryptingKey := make([]byte, keySize)
	// s2k.Serialize salts and stretches the passphrase, and writes the
	// resulting key to keyEncryptingKey and the s2k descriptor to s2kBuf.
	err = s2k.Serialize(s2kBuf, keyEncryptingKey, config.Random(), passphrase, &s2k.Config{Hash: config.Hash(), S2KCount: config.PasswordHashIterations()})
	if err != nil {
		return
	}
	s2kBytes := s2kBuf.Bytes()

	var packetLength int
	switch version {
	case 4:
		packetLength = 2 /* header */ + len(s2kBytes) + 1 /* cipher type */ + keySize
	case 5:
		nonceLen := config.AEAD().Mode().NonceLength()
		tagLen := config.AEAD().Mode().TagLength()
		packetLength = 3 + len(s2kBytes) + nonceLen + keySize + tagLen
	}
	err = serializeHeader(w, packetTypeSymmetricKeyEncrypted, packetLength)
	if err != nil {
		return
	}

	buf := make([]byte, 2)
	// Symmetric Key Encrypted Version
	buf[0] = byte(version)
	// Cipher function
	buf[1] = byte(cipherFunc)

	if version == 5 {
		// AEAD mode
		buf = append(buf, byte(config.AEAD().Mode()))
	}
	_, err = w.Write(buf)
	if err != nil {
		return
	}
	_, err = w.Write(s2kBytes)
	if err != nil {
		return
	}

	switch version {
	case 4:
		iv := make([]byte, cipherFunc.blockSize())
		c := cipher.NewCFBEncrypter(cipherFunc.new(keyEncryptingKey), iv)
		encryptedCipherAndKey := make([]byte, keySize+1)
		c.XORKeyStream(encryptedCipherAndKey, buf[1:])
		c.XORKeyStream(encryptedCipherAndKey[1:], sessionKey)
		_, err = w.Write(encryptedCipherAndKey)
		if err != nil {
			return
		}
	case 5:
		blockCipher := cipherFunc.new(keyEncryptingKey)
		mode := config.AEAD().Mode()
		aead := mode.new(blockCipher)
		// Sample nonce using random reader
		nonce := make([]byte, config.AEAD().Mode().NonceLength())
		_, err = io.ReadFull(config.Random(), nonce)
		if err != nil {
			return
		}
		// Seal and write (encryptedData includes auth. tag)
		adata := []byte{0xc3, byte(5), byte(cipherFunc), byte(mode)}
		encryptedData := aead.Seal(nil, nonce, sessionKey, adata)
		_, err = w.Write(nonce)
		if err != nil {
			return
		}
		_, err = w.Write(encryptedData)
		if err != nil {
			return
		}
	}

	return
}
