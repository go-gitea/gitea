// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"context"
	"io"
)

// BlameService provides a way to create BlameReaders
type BlameService interface {
	// CreateBlameReader creates reader for given repository, commit and file
	CreateBlameReader(ctx context.Context, repoPath, commitID, file string) (BlameReader, error)
}

// BlameReader returns part of file blame one by one
type BlameReader interface {
	io.Closer
	NextPart() (*BlamePart, error)
}

// BlamePart represents block of blame - continuous lines with one sha
type BlamePart struct {
	SHA   string
	Lines []string
}
