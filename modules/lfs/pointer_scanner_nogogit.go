// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package lfs

import (
	"io"

	"code.gitea.io/gitea/modules/git"
)

// TryReadPointer not implemented
func TryReadPointer(reader io.Reader) *Pointer {
	return nil
}

// TryReadPointerFromBuffer not implemented
func TryReadPointerFromBuffer(buf []byte) *Pointer {
	return nil
}

// SearchPointerFiles not implemented
func SearchPointerFiles(repo *git.Repository) ([]*Pointer, error) {
	return []*Pointer{}, nil
}
