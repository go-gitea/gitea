// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// TransferAdapter represents an adapter for downloading/uploading LFS objects
type TransferAdapter interface {
	Name() string
	Download(ctx context.Context, r *ObjectResponse) (io.ReadCloser, error)
	//Upload(ctx context.Context, reader io.Reader) error
}

// BasicTransferAdapter implements the "basic" adapter
type BasicTransferAdapter struct {
	client *http.Client
}

// Name returns the name of the adapter
func (a *BasicTransferAdapter) Name() string {
	return "basic"
}

// Download reads the download location and downloads the data
func (a *BasicTransferAdapter) Download(ctx context.Context, r *ObjectResponse) (io.ReadCloser, error) {
	download, ok := r.Actions["download"]
	if !ok {
		return nil, errors.New("Action 'download' not found")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", download.Href, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range download.Header {
		req.Header.Set(key, value)
	}

	res, err := a.client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, err
	}

	return res.Body, nil
}
