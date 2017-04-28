// +build !windows

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"net/http"

	"code.gitea.io/gitea/modules/log"

	"github.com/facebookgo/grace/gracehttp"
)

func runHTTP(listenAddr string, m http.Handler) error {
	return gracehttp.Serve(&http.Server{
		Addr:    listenAddr,
		Handler: m,
	})
}

func runHTTPS(listenAddr, certFile, keyFile string, m http.Handler) error {
	config := &tls.Config{
		MinVersion: tls.VersionTLS10,
	}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	config.Certificates = make([]tls.Certificate, 1)
	var err error
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal(4, "Failed to load https cert file %s: %v", listenAddr, err)
	}

	return gracehttp.Serve(&http.Server{
		Addr:      listenAddr,
		Handler:   m,
		TLSConfig: config,
	})
}
