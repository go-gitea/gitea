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

var derivedKey struct {
	sync.Mutex
	secretKey string
	key       []byte
}

// deriveKey stretches the key with Argon2id so that a guessable SECRET_KEY is costly to brute-force from a database dump
func deriveKey(secretKey string) []byte {
	derivedKey.Lock()
	defer derivedKey.Unlock()
	if derivedKey.key == nil || derivedKey.secretKey != secretKey {
		derivedKey.secretKey = secretKey
		derivedKey.key = argon2.IDKey([]byte(secretKey), []byte("gitea-secret"), 3, 64*1024, 4, 32)
	}
	return derivedKey.key
}

func EncryptSecret(key, str string) (string, error) {
	ciphertext, err := AesEncrypt(deriveKey(key), []byte(str))
	if err != nil {
		return "", fmt.Errorf("failed to encrypt by secret: %w", err)
	}
	return encryptedPrefix + hex.EncodeToString(ciphertext), nil
}

func DecryptSecret(key, encrypted string) (string, error) {
	cipherHex, ok := strings.CutPrefix(encrypted, encryptedPrefix)
	ciphertext, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt by secret, invalid hex string: %w", err)
	}
	var aesKey []byte
	if ok {
		aesKey = deriveKey(key)
	} else { // legacy value migration 356 missed, e.g. a queued task or a run under another SECRET_KEY
		legacyKey := sha256.Sum256([]byte(key))
		aesKey = legacyKey[:]
	}
	plaintext, err := AesDecrypt(aesKey, ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt by secret, the key (maybe SECRET_KEY?) might be incorrect: %w", err)
	}
	return string(plaintext), nil
}
