// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"crypto/subtle"
	"net/http"

	"code.gitea.io/gitea/modules/setting"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics validate auth token and render prometheus metrics
func Metrics(resp http.ResponseWriter, req *http.Request) {
	if setting.Metrics.Token == "" {
		promhttp.Handler().ServeHTTP(resp, req)
		return
	}
	header := req.Header.Get("Authorization")
	if header == "" {
		http.Error(resp, "", http.StatusUnauthorized)
		return
	}
	got := []byte(header)
	want := []byte("Bearer " + setting.Metrics.Token)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		http.Error(resp, "", http.StatusUnauthorized)
		return
	}
	promhttp.Handler().ServeHTTP(resp, req)
}
