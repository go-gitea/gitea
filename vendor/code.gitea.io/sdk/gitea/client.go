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
	"net/url"
	"strings"
	"sync"

	"github.com/hashicorp/go-version"
)

var jsonHeader = http.Header{"content-type": []string{"application/json"}}

// Version return the library version
func Version() string {
	return "0.15.1"
}

// Client represents a thread-safe Gitea API client.
type Client struct {
	url         string
	accessToken string
	username    string
	password    string
	otp         string
	sudo        string
	debug       bool
	client      *http.Client
	ctx         context.Context
	mutex       sync.RWMutex

	serverVersion  *version.Version
	getVersionOnce sync.Once
	ignoreVersion  bool // only set by SetGiteaVersion so don't need a mutex lock
}

// Response represents the gitea response
type Response struct {
	*http.Response
}

// ClientOption are functions used to init a new client
type ClientOption func(*Client) error

// NewClient initializes and returns a API client.
// Usage of all gitea.Client methods is concurrency-safe.
func NewClient(url string, options ...ClientOption) (*Client, error) {
	client := &Client{
		url:    strings.TrimSuffix(url, "/"),
		client: &http.Client{},
		ctx:    context.Background(),
	}
	for _, opt := range options {
		if err := opt(client); err != nil {
			return nil, err
		}
	}
	if err := client.checkServerVersionGreaterThanOrEqual(version1_11_0); err != nil {
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
func SetHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) error {
		client.SetHTTPClient(httpClient)
		return nil
	}
}

// SetHTTPClient replaces default http.Client with user given one.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.mutex.Lock()
	c.client = client
	c.mutex.Unlock()
}

// SetToken is an option for NewClient to set token
func SetToken(token string) ClientOption {
	return func(client *Client) error {
		client.mutex.Lock()
		client.accessToken = token
		client.mutex.Unlock()
		return nil
	}
}

// SetBasicAuth is an option for NewClient to set username and password
func SetBasicAuth(username, password string) ClientOption {
	return func(client *Client) error {
		client.SetBasicAuth(username, password)
		return nil
	}
}

// SetBasicAuth sets username and password
func (c *Client) SetBasicAuth(username, password string) {
	c.mutex.Lock()
	c.username, c.password = username, password
	c.mutex.Unlock()
}

// SetOTP is an option for NewClient to set OTP for 2FA
func SetOTP(otp string) ClientOption {
	return func(client *Client) error {
		client.SetOTP(otp)
		return nil
	}
}

// SetOTP sets OTP for 2FA
func (c *Client) SetOTP(otp string) {
	c.mutex.Lock()
	c.otp = otp
	c.mutex.Unlock()
}

// SetContext is an option for NewClient to set the default context
func SetContext(ctx context.Context) ClientOption {
	return func(client *Client) error {
		client.SetContext(ctx)
		return nil
	}
}

// SetContext set default context witch is used for http requests
func (c *Client) SetContext(ctx context.Context) {
	c.mutex.Lock()
	c.ctx = ctx
	c.mutex.Unlock()
}

// SetSudo is an option for NewClient to set sudo header
func SetSudo(sudo string) ClientOption {
	return func(client *Client) error {
		client.SetSudo(sudo)
		return nil
	}
}

// SetSudo sets username to impersonate.
func (c *Client) SetSudo(sudo string) {
	c.mutex.Lock()
	c.sudo = sudo
	c.mutex.Unlock()
}

// SetDebugMode is an option for NewClient to enable debug mode
func SetDebugMode() ClientOption {
	return func(client *Client) error {
		client.mutex.Lock()
		client.debug = true
		client.mutex.Unlock()
		return nil
	}
}

func (c *Client) getWebResponse(method, path string, body io.Reader) ([]byte, *Response, error) {
	c.mutex.RLock()
	debug := c.debug
	if debug {
		fmt.Printf("%s: %s\nBody: %v\n", method, c.url+path, body)
	}
	req, err := http.NewRequestWithContext(c.ctx, method, c.url+path, body)

	client := c.client // client ref can change from this point on so safe it
	c.mutex.RUnlock()

	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if debug {
		fmt.Printf("Response: %v\n\n", resp)
	}
	return data, &Response{resp}, nil
}

func (c *Client) doRequest(method, path string, header http.Header, body io.Reader) (*Response, error) {
	c.mutex.RLock()
	debug := c.debug
	if debug {
		fmt.Printf("%s: %s\nHeader: %v\nBody: %s\n", method, c.url+"/api/v1"+path, header, body)
	}
	req, err := http.NewRequestWithContext(c.ctx, method, c.url+"/api/v1"+path, body)
	if err != nil {
		c.mutex.RUnlock()
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

	client := c.client // client ref can change from this point on so safe it
	c.mutex.RUnlock()

	for k, v := range header {
		req.Header[k] = v
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if debug {
		fmt.Printf("Response: %v\n\n", resp)
	}
	return &Response{resp}, nil
}

// Converts a response for a HTTP status code indicating an error condition
// (non-2XX) to a well-known error value and response body. For non-problematic
// (2XX) status codes nil will be returned. Note that on a non-2XX response, the
// response body stream will have been read and, hence, is closed on return.
func statusCodeToErr(resp *Response) (body []byte, err error) {
	// no error
	if resp.StatusCode/100 == 2 {
		return nil, nil
	}

	//
	// error: body will be read for details
	//
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("body read on HTTP error %d: %v", resp.StatusCode, err)
	}

	switch resp.StatusCode {
	case 403:
		return data, errors.New("403 Forbidden")
	case 404:
		return data, errors.New("404 Not Found")
	case 409:
		return data, errors.New("409 Conflict")
	case 422:
		return data, fmt.Errorf("422 Unprocessable Entity: %s", string(data))
	}

	path := resp.Request.URL.Path
	method := resp.Request.Method
	header := resp.Request.Header
	errMap := make(map[string]interface{})
	if err = json.Unmarshal(data, &errMap); err != nil {
		// when the JSON can't be parsed, data was probably empty or a
		// plain string, so we try to return a helpful error anyway
		return data, fmt.Errorf("Unknown API Error: %d\nRequest: '%s' with '%s' method '%s' header and '%s' body", resp.StatusCode, path, method, header, string(data))
	}
	return data, errors.New(errMap["message"].(string))
}

func (c *Client) getResponse(method, path string, header http.Header, body io.Reader) ([]byte, *Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	// check for errors
	data, err := statusCodeToErr(resp)
	if err != nil {
		return data, resp, err
	}

	// success (2XX), read body
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
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

// pathEscapeSegments escapes segments of a path while not escaping forward slash
func pathEscapeSegments(path string) string {
	slice := strings.Split(path, "/")
	for index := range slice {
		slice[index] = url.PathEscape(slice[index])
	}
	escapedPath := strings.Join(slice, "/")
	return escapedPath
}

// escapeValidatePathSegments is a help function to validate and encode url path segments
func escapeValidatePathSegments(seg ...*string) error {
	for i := range seg {
		if seg[i] == nil || len(*seg[i]) == 0 {
			return fmt.Errorf("path segment [%d] is empty", i)
		}
		*seg[i] = url.PathEscape(*seg[i])
	}
	return nil
}
