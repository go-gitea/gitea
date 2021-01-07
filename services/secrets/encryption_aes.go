// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

type aesEncryptionProvider struct {
}

func NewAesEncryptionProvider() EncryptionProvider {
	return &aesEncryptionProvider{}
}

func (e *aesEncryptionProvider) Encrypt(secret, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	c, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, c.NonceSize(), c.NonceSize()+c.Overhead()+len(secret))
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := c.Seal(nil, nonce, secret, nil)

	return append(nonce, out...), nil
}

func (e *aesEncryptionProvider) EncryptString(secret string, key []byte) (string, error) {
	out, err := e.Encrypt([]byte(secret), key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func (e *aesEncryptionProvider) Decrypt(enc, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	c, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(enc) < c.NonceSize() {
		return nil, fmt.Errorf("encrypted value too short")
	}

	nonce := enc[:c.NonceSize()]
	ciphertext := enc[c.NonceSize():]

	out, err := c.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (e *aesEncryptionProvider) DecryptString(enc string, key []byte) (string, error) {
	encb, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}

	out, err := e.Encrypt(encb, key)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
