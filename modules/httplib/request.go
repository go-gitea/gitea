// Copyright 2013 The Beego Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var defaultTransport = sync.OnceValue(func() http.RoundTripper {
	return &http.Transport{
		Proxy:       http.ProxyFromEnvironment,
		DialContext: DialContextWithTimeout(10 * time.Second), // it is good enough in modern days
	}
})

func DialContextWithTimeout(timeout time.Duration) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, address)
	}
}

func NewRequest(url, method string) *Request {
	return &Request{
		url: url,
		req: &http.Request{
			Method:     method,
			Header:     make(http.Header),
			Proto:      "HTTP/1.1", // FIXME: from legacy httplib, it shouldn't be hardcoded
			ProtoMajor: 1,
			ProtoMinor: 1,
		},
		params: map[string]string{},

		// ATTENTION: from legacy httplib, callers must pay more attention to it, it will cause annoying bugs when the response takes a long time
		readWriteTimeout: 60 * time.Second,
	}
}

type Request struct {
	url    string
	req    *http.Request
	params map[string]string

	readWriteTimeout time.Duration
	transport        http.RoundTripper
}

// SetContext sets the request's Context
func (r *Request) SetContext(ctx context.Context) *Request {
	r.req = r.req.WithContext(ctx)
	return r
}

// SetTransport sets the request transport, if not set, will use httplib's default transport with environment proxy support
// ATTENTION: the http.Transport has a connection pool, so it should be reused as much as possible, do not create a lot of transports
func (r *Request) SetTransport(transport http.RoundTripper) *Request {
	r.transport = transport
	return r
}

func (r *Request) SetReadWriteTimeout(readWriteTimeout time.Duration) *Request {
	r.readWriteTimeout = readWriteTimeout
	return r
}

// Header set header item string in request.
func (r *Request) Header(key, value string) *Request {
	r.req.Header.Set(key, value)
	return r
}

// Param adds query param in to request.
// params build query string as ?key1=value1&key2=value2...
func (r *Request) Param(key, value string) *Request {
	r.params[key] = value
	return r
}

// Body adds request raw body. It supports string, []byte and io.Reader as body.
func (r *Request) Body(data any) *Request {
	if r == nil {
		return nil
	}
	switch t := data.(type) {
	case nil: // do nothing
	case string:
		bf := strings.NewReader(t)
		r.req.Body = io.NopCloser(bf)
		r.req.ContentLength = int64(len(t))
	case []byte:
		bf := bytes.NewBuffer(t)
		r.req.Body = io.NopCloser(bf)
		r.req.ContentLength = int64(len(t))
	case io.ReadCloser:
		r.req.Body = t
	case io.Reader:
		r.req.Body = io.NopCloser(t)
	default:
		panic(fmt.Sprintf("unsupported request body type %T", t))
	}
	return r
}

// Response executes request client and returns the response.
// Caller MUST close the response body if no error occurs.
func (r *Request) Response() (*http.Response, error) {
	var paramBody string
	if len(r.params) > 0 {
		var buf bytes.Buffer
		for k, v := range r.params {
			buf.WriteString(url.QueryEscape(k))
			buf.WriteByte('=')
			buf.WriteString(url.QueryEscape(v))
			buf.WriteByte('&')
		}
		paramBody = buf.String()
		paramBody = paramBody[0 : len(paramBody)-1]
	}

	if r.req.Method == http.MethodGet && len(paramBody) > 0 {
		if strings.Contains(r.url, "?") {
			r.url += "&" + paramBody
		} else {
			r.url = r.url + "?" + paramBody
		}
	} else if r.req.Method == http.MethodPost && r.req.Body == nil && len(paramBody) > 0 {
		r.Header("Content-Type", "application/x-www-form-urlencoded")
		r.Body(paramBody) // string
	}

	var err error
	r.req.URL, err = url.Parse(r.url)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: r.transport,
		Timeout:   r.readWriteTimeout,
	}
	if client.Transport == nil {
		client.Transport = defaultTransport()
	}

	if r.req.Header.Get("User-Agent") == "" {
		r.req.Header.Set("User-Agent", "GiteaHttpLib")
	}

	return client.Do(r.req)
}

func (r *Request) GoString() string {
	return fmt.Sprintf("%s %s", r.req.Method, r.url)
}
