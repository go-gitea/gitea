// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secrets

import (
	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/setting"
)

type plainMasterKeyProvider struct {
	key []byte
}

// NewPlainMasterKeyProvider returns unsecured static master key provider
func NewPlainMasterKeyProvider() MasterKeyProvider {
	return &plainMasterKeyProvider{}
}

// Init initializes master key provider
func (k *plainMasterKeyProvider) Init() error {
	return k.Unseal(nil)
}

// GenerateMasterKey generates a new master key and returns secret or secrets for unsealing
func (k *plainMasterKeyProvider) GenerateMasterKey() ([][]byte, error) {
	key, err := generate.NewMasterKey()
	if err != nil {
		return nil, err
	}
	k.key = key
	return [][]byte{key}, nil
}

// Unseal master key by providing unsealing secret
func (k *plainMasterKeyProvider) Unseal(secret []byte) error {
	k.key = setting.MasterKey
	return nil
}

// Seal master key
func (k *plainMasterKeyProvider) Seal() error {
	k.key = nil
	return nil
}

// IsSealed returns if master key is sealed
func (k *plainMasterKeyProvider) IsSealed() bool {
	return len(k.key) == 0
}

// GetMasterKey returns master key
func (k *plainMasterKeyProvider) GetMasterKey() ([]byte, error) {
	if k.IsSealed() {
		return nil, ErrMasterKeySealed
	}
	return k.key, nil
}
