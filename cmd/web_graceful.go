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

func runFCGI(listenAddr string, m http.Handler) error {
	// This needs to handle stdin as fcgi point
	fcgiServer := graceful.NewServer("tcp", listenAddr)

	err := fcgiServer.ListenAndServe(func(listener net.Listener) error {
		return fcgi.Serve(listener, m)
	})
	if err != nil {
		log.Fatal("Failed to start FCGI main server: %v", err)
	}
	log.Info("FCGI Listener: %s Closed", listenAddr)
	return err
}
