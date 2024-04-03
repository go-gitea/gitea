// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

// TransferAdapter represents an adapter for downloading/uploading LFS objects.
type TransferAdapter interface {
	Name() string
	Download(ctx context.Context, l *Link) (io.ReadCloser, error)
	Upload(ctx context.Context, l *Link, p Pointer, r io.Reader) error
	Verify(ctx context.Context, l *Link, p Pointer) error
}

// BasicTransferAdapter implements the "basic" adapter.
type BasicTransferAdapter struct {
	client *http.Client
}

// Name returns the name of the adapter.
func (a *BasicTransferAdapter) Name() string {
	return "basic"
}

// Download reads the download location and downloads the data.
func (a *BasicTransferAdapter) Download(ctx context.Context, l *Link) (io.ReadCloser, error) {
	req, err := createRequest(ctx, http.MethodGet, l.Href, l.Header, nil)
	if err != nil {
		return nil, err
	}
	resp, err := performRequest(ctx, a.client, req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Upload sends the content to the LFS server.
func (a *BasicTransferAdapter) Upload(ctx context.Context, l *Link, p Pointer, r io.Reader) error {
	req, err := createRequest(ctx, http.MethodPut, l.Href, l.Header, r)
	if err != nil {
		return err
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	if req.Header.Get("Transfer-Encoding") == "chunked" {
		req.TransferEncoding = []string{"chunked"}
	}
	req.ContentLength = p.Size

	res, err := performRequest(ctx, a.client, req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// Verify calls the verify handler on the LFS server
func (a *BasicTransferAdapter) Verify(ctx context.Context, l *Link, p Pointer) error {
	b, err := json.Marshal(p)
	if err != nil {
		log.Error("Error encoding json: %v", err)
		return err
	}

	req, err := createRequest(ctx, http.MethodPost, l.Href, l.Header, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", MediaType)
	res, err := performRequest(ctx, a.client, req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
