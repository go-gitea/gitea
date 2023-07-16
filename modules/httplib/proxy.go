// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// MakeReverseProxyHandler creates HTTP handler that reverse-proxies requests to destURL
func MakeReverseProxyHandler(destURL, path string, skipVerify bool) http.Handler {
	url, _ := url.Parse(destURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = url.Scheme
				req.URL.Host = url.Host
				req.URL.Path = url.Path + strings.TrimPrefix(req.URL.Path, path)
			},
			Transport: &http.Transport{
				MaxIdleConns:        5,
				IdleConnTimeout:     5 * time.Second,
				TLSHandshakeTimeout: 5 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: skipVerify,
				},
			},
			ErrorHandler: func(rw http.ResponseWriter, r *http.Request, err error) {
				rw.Header().Set("Content-Type", "text/plain")
				_, _ = rw.Write([]byte(`502 Bad Gateway`))
				rw.WriteHeader(http.StatusBadGateway)
			},
		}
		proxy.ServeHTTP(w, r)
	})
}
