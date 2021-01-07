// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secrets

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// MasterKeyProviderType is the type of master key provider
type MasterKeyProviderType string

// Types of master key providers
const (
	MasterKeyProviderTypeNone  MasterKeyProviderType = "none"
	MasterKeyProviderTypePlain MasterKeyProviderType = "plain"
)

var (
	masterKey   MasterKeyProvider
	encProvider EncryptionProvider
)

// Init initializes master key provider based on settings
func Init() error {
	switch MasterKeyProviderType(setting.MasterKeyProvider) {
	case MasterKeyProviderTypeNone:
		masterKey = NewNopMasterKeyProvider()
	case MasterKeyProviderTypePlain:
		masterKey = NewPlainMasterKeyProvider()
	default:
		return fmt.Errorf("invalid master key provider %v", setting.MasterKeyProvider)
	}

	encProvider = NewAesEncryptionProvider()

	return nil
}

// GenerateMasterKey generates a new master key and returns secret or secrets for unsealing
func GenerateMasterKey() ([][]byte, error) {
	return masterKey.GenerateMasterKey()
}

func Encrypt(secret []byte) ([]byte, error) {
	key, err := masterKey.GetMasterKey()
	if err != nil {
		return nil, err
	}

	if len(key) == 0 {
		return secret, nil
	}

	return encProvider.Encrypt(secret, key)
}

func EncryptString(secret string) (string, error) {
	key, err := masterKey.GetMasterKey()
	if err != nil {
		return "", err
	}

	if len(key) == 0 {
		return secret, nil
	}

	return encProvider.EncryptString(secret, key)
}

func Decrypt(enc []byte) ([]byte, error) {
	key, err := masterKey.GetMasterKey()
	if err != nil {
		return nil, err
	}

	if len(key) == 0 {
		return enc, nil
	}

	return encProvider.Decrypt(enc, key)
}

func DecryptString(enc string) (string, error) {
	key, err := masterKey.GetMasterKey()
	if err != nil {
		return "", err
	}

	if len(key) == 0 {
		return enc, nil
	}

	return encProvider.DecryptString(enc, key)
}
