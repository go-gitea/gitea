// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"fmt"
	"io"
	"regexp"
)

// EmptySHA defines empty git SHA
const EmptySHA = "0000000000000000000000000000000000000000"

// EmptyTreeSHA is the SHA of an empty tree
const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// SHAPattern can be used to determine if a string is an valid sha
var SHAPattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

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
