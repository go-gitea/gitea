// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/hashicorp/go-version"
)

var jsonHeader = http.Header{"content-type": []string{"application/json"}}

// Version return the library version
func Version() string {
	return "0.12.0"
}

// Client represents a Gitea API client.
type Client struct {
	url           string
	accessToken   string
	username      string
	password      string
	otp           string
	sudo          string
	client        *http.Client
	serverVersion *version.Version
	versionLock   sync.RWMutex
}

// NewClient initializes and returns a API client.
func NewClient(url, token string) *Client {
	return &Client{
		url:         strings.TrimSuffix(url, "/"),
		accessToken: token,
		client:      &http.Client{},
	}
}

// NewClientWithHTTP creates an API client with a custom http client
func NewClientWithHTTP(url string, httpClient *http.Client) *Client {
	client := NewClient(url, "")
	client.client = httpClient
	return client
}

// SetBasicAuth sets basicauth
func (c *Client) SetBasicAuth(username, password string) {
	c.username, c.password = username, password
}

// SetOTP sets OTP for 2FA
func (c *Client) SetOTP(otp string) {
	c.otp = otp
}

// SetHTTPClient replaces default http.Client with user given one.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.client = client
}

// SetSudo sets username to impersonate.
func (c *Client) SetSudo(sudo string) {
	c.sudo = sudo
}

func (c *Client) doRequest(method, path string, header http.Header, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.url+"/api/v1"+path, body)
	if err != nil {
		return nil, err
	}
	if len(c.accessToken) != 0 {
		req.Header.Set("Authorization", "token "+c.accessToken)
	}
	if len(c.otp) != 0 {
		req.Header.Set("X-GITEA-OTP", c.otp)
	}
	if len(c.username) != 0 {
		req.SetBasicAuth(c.username, c.password)
	}
	if len(c.sudo) != 0 {
		req.Header.Set("Sudo", c.sudo)
	}
	for k, v := range header {
		req.Header[k] = v
	}

	return c.client.Do(req)
}

func (c *Client) getResponse(method, path string, header http.Header, body io.Reader) ([]byte, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case 403:
		return nil, errors.New("403 Forbidden")
	case 404:
		return nil, errors.New("404 Not Found")
	case 409:
		return nil, errors.New("409 Conflict")
	case 422:
		return nil, fmt.Errorf("422 Unprocessable Entity: %s", string(data))
	}

	if resp.StatusCode/100 != 2 {
		errMap := make(map[string]interface{})
		if err = json.Unmarshal(data, &errMap); err != nil {
			// when the JSON can't be parsed, data was probably empty or a plain string,
			// so we try to return a helpful error anyway
			return nil, fmt.Errorf("Unknown API Error: %d\nRequest: '%s' with '%s' method '%s' header and '%s' body", resp.StatusCode, path, method, header, string(data))
		}
		return nil, errors.New(errMap["message"].(string))
	}

	return data, nil
}

func (c *Client) getParsedResponse(method, path string, header http.Header, body io.Reader, obj interface{}) error {
	data, err := c.getResponse(method, path, header, body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

func (c *Client) getStatusCode(method, path string, header http.Header, body io.Reader) (int, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}
