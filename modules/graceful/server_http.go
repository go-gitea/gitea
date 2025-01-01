// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
)

func newHTTPServer(network, address, name string, handler http.Handler) (*Server, ServeFunction) {
	server := NewServer(network, address, name)
	httpServer := http.Server{
		Handler:     handler,
		BaseContext: func(net.Listener) context.Context { return GetManager().HammerContext() },
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
