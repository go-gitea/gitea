// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

// EncryptionProvider encrypts and decrypts secrets
type EncryptionProvider interface {
	Encrypt(secret, key []byte) ([]byte, error)

	EncryptString(secret string, key []byte) (string, error)

	Decrypt(enc, key []byte) ([]byte, error)

	DecryptString(enc string, key []byte) (string, error)
}
