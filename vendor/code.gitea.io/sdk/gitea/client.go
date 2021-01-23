// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
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
	return "0.13.0"
}

// Client represents a Gitea API client.
type Client struct {
	url           string
	accessToken   string
	username      string
	password      string
	otp           string
	sudo          string
	debug         bool
	client        *http.Client
	ctx           context.Context
	serverVersion *version.Version
	versionLock   sync.RWMutex
}

// Response represents the gitea response
type Response struct {
	*http.Response
}

// NewClient initializes and returns a API client.
func NewClient(url string, options ...func(*Client)) (*Client, error) {
	client := &Client{
		url:    strings.TrimSuffix(url, "/"),
		client: &http.Client{},
		ctx:    context.Background(),
	}
	for _, opt := range options {
		opt(client)
	}
	if err := client.CheckServerVersionConstraint(">=1.10"); err != nil {
		return nil, err
	}
	return client, nil
}

// NewClientWithHTTP creates an API client with a custom http client
// Deprecated use SetHTTPClient option
func NewClientWithHTTP(url string, httpClient *http.Client) *Client {
	client, _ := NewClient(url, SetHTTPClient(httpClient))
	return client
}

// SetHTTPClient is an option for NewClient to set custom http client
func SetHTTPClient(httpClient *http.Client) func(client *Client) {
	return func(client *Client) {
		client.client = httpClient
	}
}

// SetToken is an option for NewClient to set token
func SetToken(token string) func(client *Client) {
	return func(client *Client) {
		client.accessToken = token
	}
}

// SetBasicAuth is an option for NewClient to set username and password
func SetBasicAuth(username, password string) func(client *Client) {
	return func(client *Client) {
		client.SetBasicAuth(username, password)
	}
}

// SetBasicAuth sets username and password
func (c *Client) SetBasicAuth(username, password string) {
	c.username, c.password = username, password
}

// SetOTP is an option for NewClient to set OTP for 2FA
func SetOTP(otp string) func(client *Client) {
	return func(client *Client) {
		client.SetOTP(otp)
	}
}

// SetOTP sets OTP for 2FA
func (c *Client) SetOTP(otp string) {
	c.otp = otp
}

// SetContext is an option for NewClient to set context
func SetContext(ctx context.Context) func(client *Client) {
	return func(client *Client) {
		client.SetContext(ctx)
	}
}

// SetContext set context witch is used for http requests
func (c *Client) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// SetHTTPClient replaces default http.Client with user given one.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.client = client
}

// SetSudo is an option for NewClient to set sudo header
func SetSudo(sudo string) func(client *Client) {
	return func(client *Client) {
		client.SetSudo(sudo)
	}
}

// SetSudo sets username to impersonate.
func (c *Client) SetSudo(sudo string) {
	c.sudo = sudo
}

// SetDebugMode is an option for NewClient to enable debug mode
func SetDebugMode() func(client *Client) {
	return func(client *Client) {
		client.debug = true
	}
}

func (c *Client) getWebResponse(method, path string, body io.Reader) ([]byte, *Response, error) {
	if c.debug {
		fmt.Printf("%s: %s\nBody: %v\n", method, c.url+path, body)
	}
	req, err := http.NewRequestWithContext(c.ctx, method, c.url+path, body)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if c.debug {
		fmt.Printf("Response: %v\n\n", resp)
	}
	return data, &Response{resp}, nil
}

func (c *Client) doRequest(method, path string, header http.Header, body io.Reader) (*Response, error) {
	if c.debug {
		fmt.Printf("%s: %s\nHeader: %v\nBody: %s\n", method, c.url+"/api/v1"+path, header, body)
	}
	req, err := http.NewRequestWithContext(c.ctx, method, c.url+"/api/v1"+path, body)
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if c.debug {
		fmt.Printf("Response: %v\n\n", resp)
	}
	return &Response{resp}, nil
}

func (c *Client) getResponse(method, path string, header http.Header, body io.Reader) ([]byte, *Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}

	switch resp.StatusCode {
	case 403:
		return data, resp, errors.New("403 Forbidden")
	case 404:
		return data, resp, errors.New("404 Not Found")
	case 409:
		return data, resp, errors.New("409 Conflict")
	case 422:
		return data, resp, fmt.Errorf("422 Unprocessable Entity: %s", string(data))
	}

	if resp.StatusCode/100 != 2 {
		errMap := make(map[string]interface{})
		if err = json.Unmarshal(data, &errMap); err != nil {
			// when the JSON can't be parsed, data was probably empty or a plain string,
			// so we try to return a helpful error anyway
			return data, resp, fmt.Errorf("Unknown API Error: %d\nRequest: '%s' with '%s' method '%s' header and '%s' body", resp.StatusCode, path, method, header, string(data))
		}
		return data, resp, errors.New(errMap["message"].(string))
	}

	return data, resp, nil
}

func (c *Client) getParsedResponse(method, path string, header http.Header, body io.Reader, obj interface{}) (*Response, error) {
	data, resp, err := c.getResponse(method, path, header, body)
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(data, obj)
}

func (c *Client) getStatusCode(method, path string, header http.Header, body io.Reader) (int, *Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return -1, resp, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, resp, nil
}
