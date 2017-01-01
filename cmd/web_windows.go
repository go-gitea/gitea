// +build windows

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"net/http"
)

func runHTTP(listenAddr string, m http.Handler) error {
	return http.ListenAndServe(listenAddr, m)
}

func runHTTPS(listenAddr, certFile, keyFile string, m http.Handler) error {
	return http.ListenAndServeTLS(listenAddr, certFile, keyFile, m)
}
