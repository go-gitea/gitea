// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/log"
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
	err := json.NewEncoder(payload).Encode(request)
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
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("lfs.HTTPClient.batch json.Decode: %w", err)
	}

	if len(response.Transfer) == 0 {
		response.Transfer = "basic"
	}

	return &response, nil
}

// Download reads the specific LFS object from the LFS server
func (c *HTTPClient) Download(ctx context.Context, oid string, size int64) (io.ReadCloser, error) {
	var objects []Pointer
	objects = append(objects, Pointer{oid, size})

	result, err := c.batch(ctx, "download", objects)
	if err != nil {
		return nil, err
	}

	transferAdapter, ok := c.transfers[result.Transfer]
	if !ok {
		return nil, fmt.Errorf("lfs.HTTPClient.Download Transferadapter not found: %s", result.Transfer)
	}

	if len(result.Objects) == 0 {
		return nil, errors.New("lfs.HTTPClient.Download: No objects in result")
	}

	content, err := transferAdapter.Download(ctx, result.Objects[0])
	if err != nil {
		return nil, err
	}
	return content, nil
}
