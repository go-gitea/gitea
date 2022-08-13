// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func newHTTPServer(network, address, name string, handler http.Handler) (*Server, ServeFunction) {
	server := NewServer(network, address, name)
	httpServer := http.Server{
		ReadTimeout:    DefaultReadTimeOut,
		WriteTimeout:   DefaultWriteTimeOut,
		MaxHeaderBytes: DefaultMaxHeaderBytes,
		Handler:        h2c.NewHandler(handler, &http2.Server{}),
		BaseContext:    func(net.Listener) context.Context { return GetManager().HammerContext() },
	}
	server.OnShutdown = func() {
		httpServer.SetKeepAlivesEnabled(false)
	}
	return server, httpServer.Serve
}

// HTTPListenAndServe listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServe(network, address, name string, handler http.Handler, useProxyProtocol bool) error {
	server, lHandler := newHTTPServer(network, address, name, handler)
	return server.ListenAndServe(lHandler, useProxyProtocol)
}

// HTTPListenAndServeTLSConfig listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServeTLSConfig(network, address, name string, tlsConfig *tls.Config, handler http.Handler, useProxyProtocol, proxyProtocolTLSBridging bool) error {
	server, lHandler := newHTTPServer(network, address, name, handler)
	return server.ListenAndServeTLSConfig(tlsConfig, lHandler, useProxyProtocol, proxyProtocolTLSBridging)
}
