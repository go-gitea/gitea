// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"net/http"

	"code.gitea.io/gitea/modules/graceful"
)

func runHTTP(listenAddr string, m http.Handler) error {
	return graceful.HTTPListenAndServe("tcp", listenAddr, m)
}

func runHTTPS(listenAddr, certFile, keyFile string, m http.Handler) error {
	return graceful.HTTPListenAndServeTLS("tcp", listenAddr, certFile, keyFile, m)
}

func runHTTPSWithTLSConfig(listenAddr string, tlsConfig *tls.Config, m http.Handler) error {
	return graceful.HTTPListenAndServeTLSConfig("tcp", listenAddr, tlsConfig, m)
}

// NoHTTPRedirector tells our cleanup routine that we will not be using a fallback http redirector
func NoHTTPRedirector() {
	graceful.Manager.InformCleanup()
}

// NoMainListener tells our cleanup routine that we will not be using a possibly provided listener
// for our main HTTP/HTTPS service
func NoMainListener() {
	graceful.Manager.InformCleanup()
}
