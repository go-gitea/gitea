// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// AesEncrypt encrypts text and given key with AES.
// It is only internally used at the moment to use "SECRET_KEY" for some database values.
func AesEncrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AesEncrypt invalid key: %v", err)
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("AesEncrypt unable to read IV: %w", err)
	}
	cfb := cipher.NewCFBEncrypter(block, iv) //nolint:staticcheck // need to migrate and refactor to a new approach
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

// AesDecrypt decrypts text and given key with AES.
// It is only internally used at the moment to use "SECRET_KEY" for some database values.
func AesDecrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("AesDecrypt ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv) //nolint:staticcheck // need to migrate and refactor to a new approach
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, fmt.Errorf("AesDecrypt invalid decrypted base64 string: %w", err)
	}
	return data, nil
}

const encryptedPrefix = "v2:"

var derivedAEAD struct {
	sync.Mutex
	secretKey string
	aead      cipher.AEAD
}

// newAEAD stretches the key with Argon2id so that a guessable SECRET_KEY is costly to brute-force from a database dump
func newAEAD(secretKey string) (cipher.AEAD, error) {
	derivedAEAD.Lock()
	defer derivedAEAD.Unlock()
	if derivedAEAD.aead == nil || derivedAEAD.secretKey != secretKey {
		block, err := aes.NewCipher(argon2.IDKey([]byte(secretKey), []byte("gitea-secret"), 3, 64*1024, 4, 32))
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCMWithRandomNonce(block)
		if err != nil {
			return nil, err
		}
		derivedAEAD.secretKey, derivedAEAD.aead = secretKey, aead
	}
	return derivedAEAD.aead, nil
}

func EncryptSecret(key, str string) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt by secret: %w", err)
	}
	return encryptedPrefix + hex.EncodeToString(aead.Seal(nil, nil, []byte(str), nil)), nil
}

func openSecret(key string, ciphertext []byte, legacy bool) ([]byte, error) {
	if legacy { // value migration 356 missed, e.g. a queued task or a run under another SECRET_KEY
		legacyKey := sha256.Sum256([]byte(key))
		return AesDecrypt(legacyKey[:], ciphertext)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nil, ciphertext, nil)
}

func DecryptSecret(key, encrypted string) (string, error) {
	cipherHex, ok := strings.CutPrefix(encrypted, encryptedPrefix)
	ciphertext, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt by secret, invalid hex string: %w", err)
	}
	plaintext, err := openSecret(key, ciphertext, !ok)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt by secret, the key (maybe SECRET_KEY?) might be incorrect: %w", err)
	}
	return string(plaintext), nil
}
