// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

var tlsVersionStringMap = map[string]uint16{
	"tlsv1.0": tls.VersionTLS10,
	"tlsv1.1": tls.VersionTLS11,
	"tlsv1.2": tls.VersionTLS12,
	"tlsv1.3": tls.VersionTLS13,
}

// toTLSVersion returns 0 when unset or unknown, which keeps the crypto/tls default
func toTLSVersion(version string) uint16 {
	version = strings.TrimSpace(strings.ToLower(version))
	tlsVersion, ok := tlsVersionStringMap[version]
	if !ok && version != "" {
		log.Warn("Unknown tls version: %s", version)
	}
	return tlsVersion
}

var curveStringMap = map[string]tls.CurveID{
	"x25519":             tls.X25519,
	"p256":               tls.CurveP256,
	"p384":               tls.CurveP384,
	"p521":               tls.CurveP521,
	"mlkem1024":          tls.MLKEM1024,
	"x25519mlkem768":     tls.X25519MLKEM768,
	"secp256r1mlkem768":  tls.SecP256r1MLKEM768,
	"secp384r1mlkem1024": tls.SecP384r1MLKEM1024,
}

// lookupAll resolves configured names, warning about and skipping the ones crypto/tls does not know
func lookupAll[T comparable](names []string, table map[string]T, kind string) []T {
	values := make([]T, 0, len(names))
	for _, name := range names {
		value, ok := table[strings.TrimSpace(strings.ToLower(name))]
		if !ok {
			log.Warn("Unknown %s: %s", kind, name)
			continue
		}
		values = append(values, value)
	}
	return values
}

// cipherStringMap is derived from crypto/tls so that suites Go adds or removes are picked up automatically
var cipherStringMap = buildCipherStringMap()

func buildCipherStringMap() map[string]uint16 {
	ciphers := map[string]uint16{
		// aliases for the two suites Go later renamed with a _SHA256 suffix
		"ecdhe_rsa_with_chacha20_poly1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		"ecdhe_ecdsa_with_chacha20_poly1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}
	for _, cipher := range append(tls.CipherSuites(), tls.InsecureCipherSuites()...) {
		ciphers[strings.ToLower(strings.TrimPrefix(cipher.Name, "TLS_"))] = cipher.ID
	}
	return ciphers
}

// applyTLSSettings applies the configured [server] TLS options. Every option
// left unset keeps the crypto/tls default, which Go keeps current.
func applyTLSSettings(tlsConfig *tls.Config) *tls.Config {
	// the listener is already TLS wrapped when it reaches http.Server, so it never sets ALPN for us
	tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2", "http/1.1")
	tlsConfig.MinVersion = toTLSVersion(setting.SSLMinimumVersion)
	tlsConfig.MaxVersion = toTLSVersion(setting.SSLMaximumVersion)

	if curves := lookupAll(setting.SSLCurvePreferences, curveStringMap, "curve"); len(curves) > 0 {
		tlsConfig.CurvePreferences = curves
	}
	if ciphers := lookupAll(setting.SSLCipherSuites, cipherStringMap, "cipher suite"); len(ciphers) > 0 {
		tlsConfig.CipherSuites = ciphers
	}
	return tlsConfig
}

// runHTTPS listens on the provided network address and then calls
// Serve to handle requests on incoming TLS connections.
//
// Filenames containing a certificate and matching private key for the server must
// be provided. If the certificate is signed by a certificate authority, the
// certFile should be the concatenation of the server's certificate followed by the
// CA's certificate.
func runHTTPS(network, listenAddr, name, certFile, keyFile string, m http.Handler, useProxyProtocol, proxyProtocolTLSBridging bool) error {
	tlsConfig := applyTLSSettings(&tls.Config{})
	tlsConfig.Certificates = make([]tls.Certificate, 1)

	certPEMBlock, err := os.ReadFile(certFile)
	if err != nil {
		log.Error("Failed to load https cert file %s for %s:%s: %v", certFile, network, listenAddr, err)
		return err
	}

	keyPEMBlock, err := os.ReadFile(keyFile)
	if err != nil {
		log.Error("Failed to load https key file %s for %s:%s: %v", keyFile, network, listenAddr, err)
		return err
	}

	tlsConfig.Certificates[0], err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		log.Error("Failed to create certificate from cert file %s and key file %s for %s:%s: %v", certFile, keyFile, network, listenAddr, err)
		return err
	}

	return graceful.HTTPListenAndServeTLSConfig(network, listenAddr, name, tlsConfig, m, useProxyProtocol, proxyProtocolTLSBridging)
}

func runHTTPSWithTLSConfig(network, listenAddr, name string, tlsConfig *tls.Config, m http.Handler, useProxyProtocol, proxyProtocolTLSBridging bool) error {
	return graceful.HTTPListenAndServeTLSConfig(network, listenAddr, name, tlsConfig, m, useProxyProtocol, proxyProtocolTLSBridging)
}
