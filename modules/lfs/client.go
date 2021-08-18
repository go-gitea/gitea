// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"context"
	"io"
	"net/url"
)

// DownloadCallback gets called for every requested LFS object to process its content
type DownloadCallback func(p Pointer, content io.ReadCloser, objectError error) error

// UploadCallback gets called for every requested LFS object to provide its content
type UploadCallback func(p Pointer, objectError error) (io.ReadCloser, error)

// Client is used to communicate with a LFS source
type Client interface {
	BatchSize() int
	Download(ctx context.Context, objects []Pointer, callback DownloadCallback) error
	Upload(ctx context.Context, objects []Pointer, callback UploadCallback) error
}

// NewClient creates a LFS client
func NewClient(endpoint *url.URL, skipTLSVerify bool) Client {
	if endpoint.Scheme == "file" {
		return newFilesystemClient(endpoint)
	}
	return newHTTPClient(endpoint, skipTLSVerify)
}
