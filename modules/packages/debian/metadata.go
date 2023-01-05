// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"errors"
)

var (
	ErrInvalidName    = errors.New("Package name is invalid")
	ErrInvalidVersion = errors.New("Package version is invalid")
	ErrInvalidArch    = errors.New("Package architecture is invalid")
)

type Package struct {
	Name     string
	Version  string
	Metadata *Metadata
}

type Metadata struct {
}
