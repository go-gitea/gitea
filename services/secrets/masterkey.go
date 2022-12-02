// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"fmt"
)

// ErrMasterKeySealed is returned when trying to use master key that is sealed
var ErrMasterKeySealed = fmt.Errorf("master key sealed")

// MasterKeyProvider provides master key used for encryption
type MasterKeyProvider interface {
	Init() error

	GenerateMasterKey() ([][]byte, error)

	Unseal(secret []byte) error

	Seal() error

	IsSealed() bool

	GetMasterKey() ([]byte, error)
}
