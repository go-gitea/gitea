// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/proxy"
)

const httpBatchSize = 20

// HTTPClient is used to communicate with the LFS server
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md
type HTTPClient struct {
	client    *http.Client
	endpoint  string
	transfers map[string]TransferAdapter
}

// BatchSize returns the preferred size of batchs to process
func (c *HTTPClient) BatchSize() int {
	return httpBatchSize
}

func newHTTPClient(endpoint *url.URL, httpTransport *http.Transport) *HTTPClient {
	if httpTransport == nil {
		httpTransport = &http.Transport{
			Proxy: proxy.Proxy(),
		}
	}

	hc := &http.Client{
		Transport: httpTransport,
	}

	basic := &BasicTransferAdapter{hc}
	client := &HTTPClient{
		client:   hc,
		endpoint: strings.TrimSuffix(endpoint.String(), "/"),
		transfers: map[string]TransferAdapter{
			basic.Name(): basic,
		},
	}

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
	log.Trace("BATCH operation with objects: %v", objects)

	url := fmt.Sprintf("%s/objects/batch", c.endpoint)

	request := &BatchRequest{operation, c.transferNames(), nil, objects}
	payload := new(bytes.Buffer)
	err := json.NewEncoder(payload).Encode(request)
	if err != nil {
		log.Error("Error encoding json: %v", err)
		return nil, err
	}

	req, err := createRequest(ctx, http.MethodPost, url, map[string]string{"Content-Type": MediaType}, payload)
	if err != nil {
		return nil, err
	}

	res, err := performRequest(ctx, c.client, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var response BatchResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		log.Error("Error decoding json: %v", err)
		return nil, err
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

	for _, object := range result.Objects {
		if object.Error != nil {
			objectError := errors.New(object.Error.Message)
			log.Trace("Error on object %v: %v", object.Pointer, objectError)
			if uc != nil {
				if _, err := uc(object.Pointer, objectError); err != nil {
					return err
				}
			} else {
				if err := dc(object.Pointer, nil, objectError); err != nil {
					return err
				}
			}
			continue
		}

		if uc != nil {
			if len(object.Actions) == 0 {
				log.Trace("%v already present on server", object.Pointer)
				continue
			}

			link, ok := object.Actions["upload"]
			if !ok {
				log.Debug("%+v", object)
				return errors.New("missing action 'upload'")
			}

			content, err := uc(object.Pointer, nil)
			if err != nil {
				return err
			}

			err = transferAdapter.Upload(ctx, link, object.Pointer, content)

			if err != nil {
				return err
			}

			link, ok = object.Actions["verify"]
			if ok {
				if err := transferAdapter.Verify(ctx, link, object.Pointer); err != nil {
					return err
				}
			}
		} else {
			link, ok := object.Actions["download"]
			if !ok {
				log.Debug("%+v", object)
				return errors.New("missing action 'download'")
			}

			content, err := transferAdapter.Download(ctx, link)
			if err != nil {
				return err
			}

			if err := dc(object.Pointer, content, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

// createRequest creates a new request, and sets the headers.
func createRequest(ctx context.Context, method, url string, headers map[string]string, body io.Reader) (*http.Request, error) {
	log.Trace("createRequest: %s", url)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		log.Error("Error creating request: %v", err)
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Accept", MediaType)

	return req, nil
}

// performRequest sends a request, optionally performs a callback on the request and returns the response.
// If the status code is 200, the response is returned, and it will contain a non-nil Body.
// Otherwise, it will return an error, and the Body will be nil or closed.
func performRequest(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	log.Trace("performRequest: %s", req.URL)
	res, err := client.Do(req)
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
		defer res.Body.Close()
		return res, handleErrorResponse(res)
	}

	return res, nil
}

func handleErrorResponse(resp *http.Response) error {
	var er ErrorResponse
	err := json.NewDecoder(resp.Body).Decode(&er)
	if err != nil {
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		log.Error("Error decoding json: %v", err)
		return err
	}

	log.Trace("ErrorResponse: %v", er)
	return errors.New(er.Message)
}
