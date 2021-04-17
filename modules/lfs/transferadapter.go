// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// TransferAdapter represents an adapter for downloading/uploading LFS objects
type TransferAdapter interface {
	Name() string
	Download(ctx context.Context, l *Link) (io.ReadCloser, error)
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
func (a *BasicTransferAdapter) Download(ctx context.Context, l *Link) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", l.Href, nil)
	if err != nil {
		return nil, fmt.Errorf("lfs.BasicTransferAdapter.Download http.NewRequestWithContext: %w", err)
	}
	for key, value := range l.Header {
		req.Header.Set(key, value)
	}

	res, err := a.client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, fmt.Errorf("lfs.BasicTransferAdapter.Download http.Do: %w", err)
	}

	return res.Body, nil
}
