// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/minio/sha256-simd"
)

// AesEncrypt encrypts text and given key with AES.
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
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

// AesDecrypt decrypts text and given key with AES.
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
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, fmt.Errorf("AesDecrypt invalid decrypted base64 string: %w", err)
	}
	return data, nil
}

// EncryptSecret encrypts a string with given key into a hex string
func EncryptSecret(key, str string) (string, error) {
	keyHash := sha256.Sum256([]byte(key))
	plaintext := []byte(str)
	ciphertext, err := AesEncrypt(keyHash[:], plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt by secret: %w", err)
	}
	return hex.EncodeToString(ciphertext), nil
}

// DecryptSecret decrypts a previously encrypted hex string
func DecryptSecret(key, cipherHex string) (string, error) {
	keyHash := sha256.Sum256([]byte(key))
	ciphertext, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt by secret, invalid hex string: %w", err)
	}
	plaintext, err := AesDecrypt(keyHash[:], ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt by secret, the key (maybe SECRET_KEY?) might be incorrect: %w", err)
	}
	return string(plaintext), nil
}
