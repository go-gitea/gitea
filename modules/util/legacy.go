// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"

	"github.com/unknwon/com" //nolint:depguard
)

// CopyFile copies file from source to target path.
func CopyFile(src, dest string) error {
	return com.Copy(src, dest)
}

// CopyDir copy files recursively from source to target directory.
// It returns error when error occurs in underlying functions.
func CopyDir(srcPath, destPath string) error {
	return com.CopyDir(srcPath, destPath)
}

// ToStr converts any interface to string. should be replaced.
func ToStr(value interface{}, args ...int) string {
	return com.ToStr(value, args...)
}

// ToSnakeCase converts a string to snake_case. should be replaced.
func ToSnakeCase(str string) string {
	return com.ToSnakeCase(str)
}

// AESGCMEncrypt (from legacy package): encrypts plaintext with the given key using AES in GCM mode. should be replaced.
func AESGCMEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

// AESGCMDecrypt (from legacy package): decrypts ciphertext with the given key using AES in GCM mode. should be replaced.
func AESGCMDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	size := gcm.NonceSize()
	if len(ciphertext)-size <= 0 {
		return nil, errors.New("ciphertext is empty")
	}

	nonce := ciphertext[:size]
	ciphertext = ciphertext[size:]

	plainText, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plainText, nil
}
