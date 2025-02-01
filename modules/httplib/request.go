// Copyright 2013 The Beego Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var defaultSetting = Settings{"GiteaServer", 60 * time.Second, 60 * time.Second, nil, nil}

// newRequest returns *Request with specific method
func newRequest(url, method string) *Request {
	var resp http.Response
	req := http.Request{
		Method:     method,
		Header:     make(http.Header),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return &Request{url, &req, map[string]string{}, defaultSetting, &resp, nil}
}

// NewRequest returns *Request with specific method
func NewRequest(url, method string) *Request {
	return newRequest(url, method)
}

// Settings is the default settings for http client
type Settings struct {
	UserAgent        string
	ConnectTimeout   time.Duration
	ReadWriteTimeout time.Duration
	TLSClientConfig  *tls.Config
	Transport        http.RoundTripper
}

// Request provides more useful methods for requesting one url than http.Request.
type Request struct {
	url     string
	req     *http.Request
	params  map[string]string
	setting Settings
	resp    *http.Response
	body    []byte
}

// SetContext sets the request's Context
func (r *Request) SetContext(ctx context.Context) *Request {
	r.req = r.req.WithContext(ctx)
	return r
}

// SetTimeout sets connect time out and read-write time out for BeegoRequest.
func (r *Request) SetTimeout(connectTimeout, readWriteTimeout time.Duration) *Request {
	r.setting.ConnectTimeout = connectTimeout
	r.setting.ReadWriteTimeout = readWriteTimeout
	return r
}

func (r *Request) SetReadWriteTimeout(readWriteTimeout time.Duration) *Request {
	r.setting.ReadWriteTimeout = readWriteTimeout
	return r
}

// SetTLSClientConfig sets tls connection configurations if visiting https url.
func (r *Request) SetTLSClientConfig(config *tls.Config) *Request {
	r.setting.TLSClientConfig = config
	return r
}

// Header add header item string in request.
func (r *Request) Header(key, value string) *Request {
	r.req.Header.Set(key, value)
	return r
}

// SetTransport sets transport to
func (r *Request) SetTransport(transport http.RoundTripper) *Request {
	r.setting.Transport = transport
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
	switch t := data.(type) {
	case nil: // do nothing
	case string:
		bf := bytes.NewBufferString(t)
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

func (r *Request) getResponse() (*http.Response, error) {
	if r.resp.StatusCode != 0 {
		return r.resp, nil
	}

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

	if r.req.Method == "GET" && len(paramBody) > 0 {
		if strings.Contains(r.url, "?") {
			r.url += "&" + paramBody
		} else {
			r.url = r.url + "?" + paramBody
		}
	} else if r.req.Method == "POST" && r.req.Body == nil && len(paramBody) > 0 {
		r.Header("Content-Type", "application/x-www-form-urlencoded")
		r.Body(paramBody) // string
	}

	var err error
	r.req.URL, err = url.Parse(r.url)
	if err != nil {
		return nil, err
	}

	trans := r.setting.Transport
	if trans == nil {
		// create default transport
		trans = &http.Transport{
			TLSClientConfig: r.setting.TLSClientConfig,
			Proxy:           http.ProxyFromEnvironment,
			DialContext:     TimeoutDialer(r.setting.ConnectTimeout),
		}
	} else if t, ok := trans.(*http.Transport); ok {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = r.setting.TLSClientConfig
		}
		if t.DialContext == nil {
			t.DialContext = TimeoutDialer(r.setting.ConnectTimeout)
		}
	}

	client := &http.Client{
		Transport: trans,
		Timeout:   r.setting.ReadWriteTimeout,
	}

	if len(r.setting.UserAgent) > 0 && len(r.req.Header.Get("User-Agent")) == 0 {
		r.req.Header.Set("User-Agent", r.setting.UserAgent)
	}

	resp, err := client.Do(r.req)
	if err != nil {
		return nil, err
	}
	r.resp = resp
	return resp, nil
}

// Response executes request client gets response manually.
// Caller MUST close the response body if no error occurs
func (r *Request) Response() (*http.Response, error) {
	return r.getResponse()
}

// TimeoutDialer returns functions of connection dialer with timeout settings for http.Transport Dial field.
func TimeoutDialer(cTimeout time.Duration) func(ctx context.Context, net, addr string) (c net.Conn, err error) {
	return func(ctx context.Context, netw, addr string) (net.Conn, error) {
		d := net.Dialer{Timeout: cTimeout}
		conn, err := d.DialContext(ctx, netw, addr)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
}

func (r *Request) GoString() string {
	return fmt.Sprintf("%s %s", r.req.Method, r.url)
}
