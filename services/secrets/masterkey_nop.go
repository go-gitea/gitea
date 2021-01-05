// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secrets

type nopMasterKeyProvider struct {
}

// NewNopMasterKeyProvider returns master key provider that holds no master key and is always unsealed
func NewNopMasterKeyProvider() MasterKeyProvider {
	return &nopMasterKeyProvider{}
}

// Init initializes master key provider
func (k *nopMasterKeyProvider) Init() error {
	return nil
}

// GenerateMasterKey always returns empty master key
func (k *nopMasterKeyProvider) GenerateMasterKey() ([][]byte, error) {
	return nil, nil
}

// Unseal master key by providing unsealing secret
func (k *nopMasterKeyProvider) Unseal(secret []byte) error {
	return nil
}

// Seal master key
func (k *nopMasterKeyProvider) Seal() error {
	return nil
}

// IsSealed always returns false
func (k *nopMasterKeyProvider) IsSealed() bool {
	return false
}

// GetMasterKey returns empty master key
func (k *nopMasterKeyProvider) GetMasterKey() ([]byte, error) {
	return nil, nil
}
