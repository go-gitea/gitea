// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"gitea.dev/modules/util"
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

// newClient creates a LFS client
func newClient(endpoint *url.URL, httpTransport *http.Transport) Client {
	if endpoint.Scheme == "file" {
		return newFilesystemClient(endpoint)
	}
	return newHTTPClient(endpoint, httpTransport)
}

// NewClientFromEndpoint creates a LFS client after resolving its endpoint.
func NewClientFromEndpoint(cloneurl, lfsurl string, httpTransport *http.Transport) (Client, error) {
	endpoint := DetermineEndpoint(cloneurl, lfsurl)
	if endpoint == nil {
		source := cloneurl
		if lfsurl != "" {
			source = lfsurl
		}
		return nil, fmt.Errorf("unable to determine LFS endpoint from %q", util.SanitizeCredentialURLs(source))
	}
	return newClient(endpoint, httpTransport), nil
}
