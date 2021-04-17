// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"context"
	"io"
	"net/url"
)

// Client is used to communicate with a LFS source
type Client interface {
	Download(ctx context.Context, p Pointer) (io.ReadCloser, error)
	Upload(ctx context.Context, p Pointer, content io.Reader) error
}

// NewClient creates a LFS client
func NewClient(endpoint *url.URL) Client {
	if endpoint.Scheme == "file" {
		return newFilesystemClient(endpoint)
	}
	return newHTTPClient(endpoint)
}
