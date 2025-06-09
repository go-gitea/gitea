// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net"
	"net/http"
	"net/http/fcgi"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func runHTTP(network, listenAddr, name string, m http.Handler, useProxyProtocol bool) error {
	return graceful.HTTPListenAndServe(network, listenAddr, name, m, useProxyProtocol)
}

// NoHTTPRedirector tells our cleanup routine that we will not be using a fallback http redirector
func NoHTTPRedirector() {
	graceful.GetManager().InformCleanup()
}

// NoInstallListener tells our cleanup routine that we will not be using a possibly provided listener
// for our install HTTP/HTTPS service
func NoInstallListener() {
	graceful.GetManager().InformCleanup()
}

func runFCGI(network, listenAddr, name string, m http.Handler, useProxyProtocol bool) error {
	// This needs to handle stdin as fcgi point
	fcgiServer := graceful.NewServer(network, listenAddr, name)

	err := fcgiServer.ListenAndServe(func(listener net.Listener) error {
		return fcgi.Serve(listener, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if setting.AppSubURL != "" {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, setting.AppSubURL)
			}
			m.ServeHTTP(resp, req)
		}))
	}, useProxyProtocol)
	if err != nil {
		log.Fatal("Failed to start FCGI main server: %v", err)
	}
	log.Info("FCGI Listener: %s Closed", listenAddr)
	return err
}
