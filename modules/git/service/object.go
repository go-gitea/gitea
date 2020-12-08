// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"context"
	"io"
)

// Object represents a git object
type Object interface {
	// Type returns the type of object this is
	Type() ObjectType

	// ID returns the hash this object is associated with
	ID() Hash

	// Size returns the size of the object
	Size() int64

	// Reader returns a ReadCloser
	Reader() (io.ReadCloser, error)

	// ReaderContext returns a ReadCloser
	ReaderContext(ctx context.Context) (io.ReadCloser, error)

	// Repository returns the repository this object is associated with
	Repository() Repository
}
