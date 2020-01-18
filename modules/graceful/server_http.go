// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"crypto/tls"
	"net/http"
)

func newHTTPServer(network, address string, handler http.Handler) (*Server, ServeFunction) {
	server := NewServer(network, address)
	httpServer := http.Server{
		ReadTimeout:    DefaultReadTimeOut,
		WriteTimeout:   DefaultWriteTimeOut,
		MaxHeaderBytes: DefaultMaxHeaderBytes,
		Handler:        handler,
	}
	server.OnShutdown = func() {
		httpServer.SetKeepAlivesEnabled(false)
	}
	return server, httpServer.Serve
}

// HTTPListenAndServe listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServe(network, address string, handler http.Handler) error {
	server, lHandler := newHTTPServer(network, address, handler)
	return server.ListenAndServe(lHandler)
}

// HTTPListenAndServeTLS listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServeTLS(network, address, certFile, keyFile string, handler http.Handler) error {
	server, lHandler := newHTTPServer(network, address, handler)
	return server.ListenAndServeTLS(certFile, keyFile, lHandler)
}

// HTTPListenAndServeTLSConfig listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServeTLSConfig(network, address string, tlsConfig *tls.Config, handler http.Handler) error {
	server, lHandler := newHTTPServer(network, address, handler)
	return server.ListenAndServeTLSConfig(tlsConfig, lHandler)
}
