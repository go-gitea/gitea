// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package proxy

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

// httpProxy is a HTTPS connect proxy.
type httpProxy struct {
	host     string
	haveAuth bool
	username string
	password string
	forward  proxy.Dialer
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}

	return s, nil
}

// Dial  dials with http CONNECT method
func (s *httpProxy) Dial(network, addr string) (c net.Conn, err error) {
	// Dial and create the https client connection.
	succeed := false
	defer func() {
		if !succeed && c != nil {
			c.Close()
		}
	}()

	c, err = s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	// HACK. http.ReadRequest also does this.
	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		return nil, err
	}
	reqURL.Scheme = ""

	req, err := http.NewRequest("CONNECT", reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Close = false

	if s.haveAuth {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("User-Agent", "Gitea Client")

	err = req.Write(c)
	if err != nil {
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		err = fmt.Errorf("connect server using proxy error, StatusCode [%d]", resp.StatusCode)
		return nil, err
	}

	succeed = true
	return c, nil
}
