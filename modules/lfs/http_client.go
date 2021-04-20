// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/log"

	jsoniter "github.com/json-iterator/go"
)

const batchSize = 20

// HTTPClient is used to communicate with the LFS server
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md
type HTTPClient struct {
	client    *http.Client
	endpoint  string
	transfers map[string]TransferAdapter
}

// BatchSize returns the preferred size of batchs to process
func (c *HTTPClient) BatchSize() int {
	return batchSize
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

	log.Trace("lfs.HTTPClient.batch calling: %s", url)

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
func (c *HTTPClient) Download(ctx context.Context, objects []Pointer, callback DownloadCallback) error {
	return c.performOperation(ctx, objects, callback, nil)
}

// Upload sends the specific LFS object to the LFS server
func (c *HTTPClient) Upload(ctx context.Context, objects []Pointer, callback UploadCallback) error {
	return c.performOperation(ctx, objects, nil, callback)
}

func (c *HTTPClient) performOperation(ctx context.Context, objects []Pointer, dc DownloadCallback, uc UploadCallback) error {
	if len(objects) == 0 {
		return nil
	}

	operation := "download"
	if uc != nil {
		operation = "upload"
	}

	result, err := c.batch(ctx, operation, objects)
	if err != nil {
		return err
	}

	transferAdapter, ok := c.transfers[result.Transfer]
	if !ok {
		return fmt.Errorf("TransferAdapter not found: %s", result.Transfer)
	}

	if uc != nil {
		for _, object := range result.Objects {
			p := Pointer{object.Oid, object.Size}

			if object.Error != nil {
				if _, err := uc(p, errors.New(object.Error.Message)); err != nil {
					return err
				}
				continue
			}

			if len(object.Actions) == 0 {
				continue
			}

			link, ok := object.Actions["upload"]
			if !ok {
				return errors.New("Action 'upload' not found")
			}

			content, err := uc(p, nil)
			if err != nil {
				return err
			}

			err = transferAdapter.Upload(ctx, link, p, content)

			content.Close()

			if err != nil {
				return err
			}

			link, ok = object.Actions["verify"]
			if ok {
				if err := transferAdapter.Verify(ctx, link, p); err != nil {
					return err
				}
			}
		}
	} else {
		for _, object := range result.Objects {
			p := Pointer{object.Oid, object.Size}

			if object.Error != nil {
				if err := dc(p, nil, errors.New(object.Error.Message)); err != nil {
					return err
				}
				continue
			}

			link, ok := object.Actions["download"]
			if !ok {
				return errors.New("Action 'download' not found")
			}

			content, err := transferAdapter.Download(ctx, link)
			if err != nil {
				return err
			}

			if err := dc(p, content, nil); err != nil {
				return err
			}
		}
	}

	return nil
}
