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
	masterKey MasterKeyProvider
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
	return nil
}

// GenerateMasterKey generates a new master key and returns secret or secrets for unsealing
func GenerateMasterKey() ([][]byte, error) {
	return masterKey.GenerateMasterKey()
}
