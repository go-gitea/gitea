// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/klauspost/cpuid/v2"
)

func toTLSVersion(version string) uint16 {
	switch strings.TrimSpace(strings.ToLower(version)) {
	case "tlsv1.0":
		return tls.VersionTLS10
	case "tlsv1.1":
		return tls.VersionTLS11
	case "tlsv1.2":
		return tls.VersionTLS12
	case "tlsv1.3":
		return tls.VersionTLS13
	default:
		log.Warn("Unknown tls version: %s", version)
		return 0
	}
}

func toCurvePreferences(preferences []string) []tls.CurveID {
	ids := make([]tls.CurveID, 0, len(preferences))
	for _, pref := range preferences {
		var id tls.CurveID
		switch strings.TrimSpace(strings.ToLower(pref)) {
		case "x25519":
			id = tls.X25519
		case "p256":
			id = tls.CurveP256
		case "p384":
			id = tls.CurveP384
		case "p521":
			id = tls.CurveP521
		default:
			log.Warn("Unknown curve: %s", pref)
		}
		if id != 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func toTLSCiphers(cipherStrings []string) []uint16 {
	ciphers := make([]uint16, 0, len(cipherStrings))
	for _, cipherString := range cipherStrings {
		var cipher uint16
		switch strings.TrimSpace(strings.ToLower(cipherString)) {
		case "rsa_with_rc4_128_sha":
			cipher = tls.TLS_RSA_WITH_RC4_128_SHA
		case "rsa_with_3des_ede_cbc_sha":
			cipher = tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA
		case "rsa_with_aes_128_cbc_sha":
			cipher = tls.TLS_RSA_WITH_AES_128_CBC_SHA
		case "rsa_with_aes_256_cbc_sha":
			cipher = tls.TLS_RSA_WITH_AES_256_CBC_SHA
		case "rsa_with_aes_128_cbc_sha256":
			cipher = tls.TLS_RSA_WITH_AES_128_CBC_SHA256
		case "rsa_with_aes_128_gcm_sha256":
			cipher = tls.TLS_RSA_WITH_AES_128_GCM_SHA256
		case "rsa_with_aes_256_gcm_sha384":
			cipher = tls.TLS_RSA_WITH_AES_256_GCM_SHA384
		case "ecdhe_ecdsa_with_rc4_128_sha":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA
		case "ecdhe_ecdsa_with_aes_128_cbc_sha":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA
		case "ecdhe_ecdsa_with_aes_256_cbc_sha":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA
		case "ecdhe_rsa_with_rc4_128_sha":
			cipher = tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA
		case "ecdhe_rsa_with_3des_ede_cbc_sha":
			cipher = tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
		case "ecdhe_rsa_with_aes_128_cbc_sha":
			cipher = tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA
		case "ecdhe_rsa_with_aes_256_cbc_sha":
			cipher = tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
		case "ecdhe_ecdsa_with_aes_128_cbc_sha256":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256
		case "ecdhe_rsa_with_aes_128_cbc_sha256":
			cipher = tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256
		case "ecdhe_rsa_with_aes_128_gcm_sha256":
			cipher = tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
		case "ecdhe_ecdsa_with_aes_128_gcm_sha256":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
		case "ecdhe_rsa_with_aes_256_gcm_sha384":
			cipher = tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
		case "ecdhe_ecdsa_with_aes_256_gcm_sha384":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
		case "ecdhe_rsa_with_chacha20_poly1305_sha256":
			cipher = tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
		case "ecdhe_ecdsa_with_chacha20_poly1305_sha256":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256
		case "ecdhe_rsa_with_chacha20_poly1305":
			cipher = tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
		case "ecdhe_ecdsa_with_chacha20_poly1305":
			cipher = tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
		case "aes_128_gcm_sha256":
			cipher = tls.TLS_AES_128_GCM_SHA256
		case "aes_256_gcm_sha384":
			cipher = tls.TLS_AES_256_GCM_SHA384
		case "chacha20_poly1305_sha256":
			cipher = tls.TLS_CHACHA20_POLY1305_SHA256
		default:
			log.Warn("Unknown cipher: %s", cipherString)
		}
		if cipher != 0 {
			ciphers = append(ciphers, cipher)
		}
	}

	return ciphers
}

// defaultCiphers uses hardware support to check if AES is specifically
// supported by the CPU.
//
// If it is AES ciphers will be preferred over ChaCha based ciphers
func defaultCiphers() []uint16 {
	if cpuid.CPU.Supports(cpuid.AESNI) {
		return defaultCiphersAESfirst
	}
	return defaultCiphersChaChaFirst
}

var (
	defaultCiphersAES = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}

	defaultCiphersChaCha = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	}

	defaultCiphersAESfirst    = append(defaultCiphersAES, defaultCiphersChaCha...)
	defaultCiphersChaChaFirst = append(defaultCiphersChaCha, defaultCiphersAES...)
)

// runHTTPs listens on the provided network address and then calls
// Serve to handle requests on incoming TLS connections.
//
// Filenames containing a certificate and matching private key for the server must
// be provided. If the certificate is signed by a certificate authority, the
// certFile should be the concatenation of the server's certificate followed by the
// CA's certificate.
func runHTTPS(network, listenAddr, name, certFile, keyFile string, m http.Handler) error {
	tlsConfig := &tls.Config{}
	if tlsConfig.NextProtos == nil {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}

	if version := toTLSVersion(setting.SSLMinimumVersion); version != 0 {
		tlsConfig.MinVersion = version
	}
	if version := toTLSVersion(setting.SSLMaximumVersion); version != 0 {
		tlsConfig.MaxVersion = version
	}

	// Set curve preferences
	tlsConfig.CurvePreferences = []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
	}
	if curves := toCurvePreferences(setting.SSLCurvePreferences); len(curves) > 0 {
		tlsConfig.CurvePreferences = curves
	}

	// Set cipher suites
	tlsConfig.CipherSuites = defaultCiphers()
	if ciphers := toTLSCiphers(setting.SSLCipherSuites); len(ciphers) > 0 {
		tlsConfig.CipherSuites = ciphers
	}

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

	return graceful.HTTPListenAndServeTLSConfig(network, listenAddr, name, tlsConfig, m)
}

func runHTTPSWithTLSConfig(network, listenAddr, name string, tlsConfig *tls.Config, m http.Handler) error {
	return graceful.HTTPListenAndServeTLSConfig(network, listenAddr, name, tlsConfig, m)
}
