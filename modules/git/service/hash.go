// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"fmt"
	"io"
)

// Hash represents a Git Hash compatible with the repository
type Hash interface {
	fmt.Stringer
	IsZero() bool
	FromString(idStr string) (Hash, error)
}

// HashComputer takes a type and computes a hash from the reader
type HashComputer interface {
	ComputeHash(t ObjectType, rd *io.Reader) Hash
}
