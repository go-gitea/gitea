// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/fcgi"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
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
	graceful.GetManager().InformCleanup()
}

// NoMainListener tells our cleanup routine that we will not be using a possibly provided listener
// for our main HTTP/HTTPS service
func NoMainListener() {
	graceful.GetManager().InformCleanup()
}

func runFCGI(network, listenAddr string, m http.Handler) error {
	// This needs to handle stdin as fcgi point
	fcgiServer := graceful.NewServer(network, listenAddr)

	err := fcgiServer.ListenAndServe(func(listener net.Listener) error {
		return fcgi.Serve(listener, m)
	})
	if err != nil {
		log.Fatal("Failed to start FCGI main server: %v", err)
	}
	log.Info("FCGI Listener: %s Closed", listenAddr)
	return err
}
