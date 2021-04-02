// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package io

import "io"

// ReadSizer represents a reader which have size information
type ReadSizer interface {
	io.Reader
	Size() int64
}

type readSizer struct {
	io.Reader
	size int64
}

func (r *readSizer) Size() int64 {
	return r.size
}

// WithSize binding io.Reader and size into a ReadSizer
func WithSize(rd io.Reader, size int64) ReadSizer {
	return &readSizer{
		Reader: rd,
		size:   size,
	}
}
