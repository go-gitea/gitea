// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package lfs

import (
	"io"

	"code.gitea.io/gitea/modules/git"
)

func TryReadPointer(reader io.Reader) *Pointer {
	return nil
}

func TryReadPointerFromBuffer(buf []byte) *Pointer {
	return nil
}

func SearchPointerFiles(repo *git.Repository) ([]*Pointer, error) {
	return []*Pointer{}, nil
}