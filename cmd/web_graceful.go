// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"net/http"

	"code.gitea.io/gitea/modules/graceful"
)

func runHTTP(network, listenAddr string, m http.Handler) error {
	return graceful.HTTPListenAndServe(network, listenAddr, m)
}

func runHTTPS(network, listenAddr, certFile, keyFile string, m http.Handler) error {
	return graceful.HTTPListenAndServeTLS(network, listenAddr, certFile, keyFile, m)
}

func runHTTPSWithTLSConfig(network, listenAddr string, tlsConfig *tls.Config, m http.Handler) error {
	return graceful.HTTPListenAndServeTLSConfig(network, listenAddr, tlsConfig, m)
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
