// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/caddyserver/certmagic"
)

func getCARoot(path string) (*x509.CertPool, error) {
	r, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(r)
	if block == nil {
		return nil, fmt.Errorf("no PEM found in the file %s", path)
	}
	caRoot, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(caRoot)
	return certPool, nil
}

func runACME(listenAddr string, m http.Handler) error {
	// If HTTP Challenge enabled, needs to be serving on port 80. For TLSALPN needs 443.
	// Due to docker port mapping this can't be checked programmatically
	// TODO: these are placeholders until we add options for each in settings with appropriate warning
	enableHTTPChallenge := true
	enableTLSALPNChallenge := true
	altHTTPPort := 0
	altTLSALPNPort := 0

	if p, err := strconv.Atoi(setting.PortToRedirect); err == nil {
		altHTTPPort = p
	}
	if p, err := strconv.Atoi(setting.HTTPPort); err == nil {
		altTLSALPNPort = p
	}

	magic := &certmagic.Default
	magic.Storage = &certmagic.FileStorage{Path: setting.AcmeLiveDirectory}
	// Try to use private CA root if provided, otherwise defaults to system's trust
	var certPool *x509.CertPool
	if setting.AcmeCARoot != "" {
		var err error
		certPool, err = getCARoot(setting.AcmeCARoot)
		if err != nil {
			log.Warn("Failed to parse CA Root certificate, using default CA trust: %v", err)
		}
	}
	myACME := certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
		CA:                      setting.AcmeURL,
		TrustedRoots:            certPool,
		Email:                   setting.AcmeEmail,
		Agreed:                  setting.AcmeTOS,
		DisableHTTPChallenge:    !enableHTTPChallenge,
		DisableTLSALPNChallenge: !enableTLSALPNChallenge,
		ListenHost:              setting.HTTPAddr,
		AltTLSALPNPort:          altTLSALPNPort,
		AltHTTPPort:             altHTTPPort,
	})

	magic.Issuers = []certmagic.Issuer{myACME}

	// this obtains certificates or renews them if necessary
	err := magic.ManageSync(graceful.GetManager().HammerContext(), []string{setting.Domain})
	if err != nil {
		return err
	}

	tlsConfig := magic.TLSConfig()
	tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")

	if version := toTLSVersion(setting.SSLMinimumVersion); version != 0 {
		tlsConfig.MinVersion = version
	}
	if version := toTLSVersion(setting.SSLMaximumVersion); version != 0 {
		tlsConfig.MaxVersion = version
	}

	// Set curve preferences
	if curves := toCurvePreferences(setting.SSLCurvePreferences); len(curves) > 0 {
		tlsConfig.CurvePreferences = curves
	}

	// Set cipher suites
	if ciphers := toTLSCiphers(setting.SSLCipherSuites); len(ciphers) > 0 {
		tlsConfig.CipherSuites = ciphers
	}

	if enableHTTPChallenge {
		go func() {
			_, _, finished := process.GetManager().AddTypedContext(graceful.GetManager().HammerContext(), "Web: ACME HTTP challenge server", process.SystemProcessType, true)
			defer finished()

			log.Info("Running Let's Encrypt handler on %s", setting.HTTPAddr+":"+setting.PortToRedirect)
			// all traffic coming into HTTP will be redirect to HTTPS automatically (LE HTTP-01 validation happens here)
			err := runHTTP("tcp", setting.HTTPAddr+":"+setting.PortToRedirect, "Let's Encrypt HTTP Challenge", myACME.HTTPChallengeHandler(http.HandlerFunc(runLetsEncryptFallbackHandler)), setting.RedirectorUseProxyProtocol)
			if err != nil {
				log.Fatal("Failed to start the Let's Encrypt handler on port %s: %v", setting.PortToRedirect, err)
			}
		}()
	}

	return runHTTPSWithTLSConfig("tcp", listenAddr, "Web", tlsConfig, m, setting.UseProxyProtocol, setting.ProxyProtocolTLSBridging)
}

func runLetsEncryptFallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Use HTTPS", http.StatusBadRequest)
		return
	}
	// Remove the trailing slash at the end of setting.AppURL, the request
	// URI always contains a leading slash, which would result in a double
	// slash
	target := strings.TrimSuffix(setting.AppURL, "/") + r.URL.RequestURI()
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}
