// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func MakeDevAssetsHandler(destUrl, path string) http.Handler {
	url, _ := url.Parse(destUrl)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = url.Scheme
				req.URL.Host = url.Host
				req.URL.Path = url.Path + strings.TrimPrefix(req.URL.Path, path)
			},
			Transport: &http.Transport{
				MaxIdleConns:    100,
				IdleConnTimeout: 5 * time.Second,
			},
			ErrorHandler: func(rw http.ResponseWriter, r *http.Request, err error) {
				rw.Header().Set("Content-Type", "text/html")
				if _, err := rw.Write([]byte(`<html><head><title>502 Bad Gateway</title></head><body><center><h1>502 Bad Gateway</h1></center><hr><center>gitea</center></body></html>`)); err != nil {
				}
				rw.WriteHeader(http.StatusBadGateway)
			},
		}
		proxy.ServeHTTP(w, r)
	})
}
