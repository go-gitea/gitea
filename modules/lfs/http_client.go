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
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/log"

	jsoniter "github.com/json-iterator/go"
)

// HTTPClient is used to communicate with the LFS server
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md
type HTTPClient struct {
	client    *http.Client
	endpoint  string
	transfers map[string]TransferAdapter
}

func newHTTPClient(endpoint *url.URL) *HTTPClient {
	hc := &http.Client{}

	client := &HTTPClient{
		client:    hc,
		endpoint:  strings.TrimSuffix(endpoint.String(), "/"),
		transfers: make(map[string]TransferAdapter),
	}

	basic := &BasicTransferAdapter{hc}

	client.transfers[basic.Name()] = basic

	return client
}

func (c *HTTPClient) transferNames() []string {
	keys := make([]string, len(c.transfers))

	i := 0
	for k := range c.transfers {
		keys[i] = k
		i++
	}

	return keys
}

func (c *HTTPClient) batch(ctx context.Context, operation string, objects []Pointer) (*BatchResponse, error) {
	url := fmt.Sprintf("%s/objects/batch", c.endpoint)

	request := &BatchRequest{operation, c.transferNames(), nil, objects}

	payload := new(bytes.Buffer)
	err := jsoniter.NewEncoder(payload).Encode(request)
	if err != nil {
		return nil, fmt.Errorf("lfs.HTTPClient.batch json.Encode: %w", err)
	}

	log.Trace("lfs.HTTPClient.batch NewRequestWithContext: %s", url)

	req, err := http.NewRequestWithContext(ctx, "POST", url, payload)
	if err != nil {
		return nil, fmt.Errorf("lfs.HTTPClient.batch http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Content-type", MediaType)
	req.Header.Set("Accept", MediaType)

	res, err := c.client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, fmt.Errorf("lfs.HTTPClient.batch http.Do: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lfs.HTTPClient.batch: Unexpected servers response: %s", res.Status)
	}

	var response BatchResponse
	err = jsoniter.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("lfs.HTTPClient.batch json.Decode: %w", err)
	}

	if len(response.Transfer) == 0 {
		response.Transfer = "basic"
	}

	return &response, nil
}

// Download reads the specific LFS object from the LFS server
func (c *HTTPClient) Download(ctx context.Context, p Pointer) (io.ReadCloser, error) {
	bc := batchContext{
		IsUpload: false,
		Pointer:  p,
	}
	err := c.performOperation(ctx, &bc)
	if err != nil {
		return nil, err
	}
	return bc.DownloadResult, nil
}

// Upload sends the specific LFS object to the LFS server
func (c *HTTPClient) Upload(ctx context.Context, p Pointer, r io.Reader) error {
	bc := batchContext{
		IsUpload:      true,
		Pointer:       p,
		UploadContent: r,
	}
	return c.performOperation(ctx, &bc)
}

type batchContext struct {
	Pointer
	IsUpload bool

	DownloadResult io.ReadCloser
	UploadContent  io.Reader
}

func (c *HTTPClient) performOperation(ctx context.Context, bc *batchContext) error {
	operation := "download"
	if bc.IsUpload {
		operation = "upload"
	}

	result, err := c.batch(ctx, operation, []Pointer{bc.Pointer})
	if err != nil {
		return err
	}

	transferAdapter, ok := c.transfers[result.Transfer]
	if !ok {
		return fmt.Errorf("TransferAdapter not found: %s", result.Transfer)
	}

	if len(result.Objects) == 0 {
		return errors.New("No objects in result")
	}

	object := result.Objects[0]

	if object.Error != nil {
		return errors.New(object.Error.Message)
	}

	if bc.IsUpload {
		if len(object.Actions) == 0 {
			return nil
		}

		link, ok := object.Actions["upload"]
		if !ok {
			return errors.New("Action 'upload' not found")
		}

		if err := transferAdapter.Upload(ctx, link, bc.UploadContent); err != nil {
			return err
		}

		link, ok = object.Actions["verify"]
		if ok {
			if err := transferAdapter.Verify(ctx, link, bc.Pointer); err != nil {
				return err
			}
		}
	} else {
		link, ok := object.Actions["download"]
		if !ok {
			return errors.New("Action 'download' not found")
		}

		var err error
		bc.DownloadResult, err = transferAdapter.Download(ctx, link)
		if err != nil {
			return err
		}
	}
	return nil
}
