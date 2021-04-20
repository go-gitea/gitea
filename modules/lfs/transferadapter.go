// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/log"

	jsoniter "github.com/json-iterator/go"
)

// TransferAdapter represents an adapter for downloading/uploading LFS objects
type TransferAdapter interface {
	Name() string
	Download(ctx context.Context, l *Link) (io.ReadCloser, error)
	Upload(ctx context.Context, l *Link, p Pointer, r io.Reader) error
	Verify(ctx context.Context, l *Link, p Pointer) error
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
	resp, err := a.performRequest(ctx, "GET", l, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Upload sends the content to the LFS server
func (a *BasicTransferAdapter) Upload(ctx context.Context, l *Link, p Pointer, r io.Reader) error {
	_, err := a.performRequest(ctx, "PUT", l, r, func(req *http.Request) {
		if len(req.Header.Get("Content-Type")) == 0 {
			req.Header.Set("Content-Type", "application/octet-stream")
		}

		if req.Header.Get("Transfer-Encoding") == "chunked" {
			req.TransferEncoding = []string{"chunked"}
		}

		req.ContentLength = p.Size
	})
	if err != nil {
		return err
	}
	return nil
}

// Verify calls the verify handler on the LFS server
func (a *BasicTransferAdapter) Verify(ctx context.Context, l *Link, p Pointer) error {
	b, err := jsoniter.Marshal(p)
	if err != nil {
		log.Error("Error encoding json: %v", err)
		return err
	}

	_, err = a.performRequest(ctx, "POST", l, bytes.NewReader(b), func(req *http.Request) {
		req.Header.Set("Content-Type", MediaType)
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *BasicTransferAdapter) performRequest(ctx context.Context, method string, l *Link, body io.Reader, callback func(*http.Request)) (*http.Response, error) {
	log.Trace("Calling: %s %s", method, l.Href)

	req, err := http.NewRequestWithContext(ctx, method, l.Href, body)
	if err != nil {
		log.Error("Error creating request: %v", err)
		return nil, err
	}
	for key, value := range l.Header {
		req.Header.Set(key, value)
	}
	req.Header.Set("Accept", MediaType)

	if callback != nil {
		callback(req)
	}

	res, err := a.client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}
		log.Error("Error while processing request: %v", err)
		return res, err
	}

	if res.StatusCode != http.StatusOK {
		return res, handleErrorResponse(res)
	}

	return res, nil
}

func handleErrorResponse(resp *http.Response) error {
	defer resp.Body.Close()

	er, err := decodeReponseError(resp.Body)
	if err != nil {
		return fmt.Errorf("Request failed with status %s", resp.Status)
	}
	log.Trace("ErrorRespone: %v", er)
	return errors.New(er.Message)
}

func decodeReponseError(r io.Reader) (ErrorResponse, error) {
	var er ErrorResponse
	err := jsoniter.NewDecoder(r).Decode(&er)
	if err != nil {
		log.Error("Error decoding json: %v", err)
	}
	return er, err
}
