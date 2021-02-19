// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is used to communicate with the LFS server
type Client struct {
	client    *http.Client
	transfers map[string]TransferAdapter
}

// NewClient creates a LFS client
func NewClient(hc *http.Client) *Client {
	client := &Client{hc, make(map[string]TransferAdapter)}

	basic := &BasicTransferAdapter{hc}

	client.transfers[basic.Name()] = basic

	return client
}

func (c *Client) transferNames() []string {
	keys := make([]string, len(c.transfers))

	i := 0
	for k := range c.transfers {
		keys[i] = k
		i++
	}

	return keys
}

func (c *Client) batch(url, operation string, objects []*Pointer) (*BatchResponse, error) {
	url = fmt.Sprintf("%s.git/info/lfs/objects/batch", strings.TrimSuffix(url, ".git"))

	request := &BatchRequest{operation, c.transferNames(), nil, objects}

	payload := new(bytes.Buffer)
	err := json.NewEncoder(payload).Encode(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-type", MediaType)
	req.Header.Set("Accept", MediaType)

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected servers response: %s", res.Status)
	}

	var response BatchResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if len(response.Transfer) == 0 {
		response.Transfer = "basic"
	}

	return &response, nil
}

// Download reads the specific LFS object from the LFS server
func (c *Client) Download(url, oid string, size int64) (io.ReadCloser, error) {
	var objects []*Pointer
	objects = append(objects, &Pointer{oid, size})

	result, err := c.batch(url, "download", objects)
	if err != nil {
		return nil, err
	}

	transferAdapter, ok := c.transfers[result.Transfer]
	if !ok {
		return nil, fmt.Errorf("Transferadapter not found: %s", result.Transfer)
	}

	if len(result.Objects) == 0 {
		return nil, errors.New("No objects in result")
	}

	content, err := transferAdapter.Download(result.Objects[0])
	if err != nil {
		return nil, err
	}
	return content, nil
}
