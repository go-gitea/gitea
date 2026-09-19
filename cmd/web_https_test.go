// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"crypto/tls"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToTLSVersion(t *testing.T) {
	assert.Zero(t, toTLSVersion(""), "unset must keep the crypto/tls default")
	assert.Zero(t, toTLSVersion("unknown"))
	assert.Equal(t, uint16(tls.VersionTLS12), toTLSVersion(" TLSV1.2 "))
}

func unsetTLSSettings(t *testing.T) {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.SSLMinimumVersion, ""))
	t.Cleanup(test.MockVariableValue(&setting.SSLMaximumVersion, ""))
	t.Cleanup(test.MockVariableValue(&setting.SSLCurvePreferences, nil))
	t.Cleanup(test.MockVariableValue(&setting.SSLCipherSuites, nil))
}

// TestApplyTLSSettingsUnsetKeepsGoDefaults guards against reintroducing hardcoded defaults,
// which is how the server ended up capped at TLS 1.2 and without post-quantum key exchange.
// It asserts the fields are left alone rather than which algorithms Go then picks, so a new
// Go release changing its preferences does not fail this test.
func TestApplyTLSSettingsUnsetKeepsGoDefaults(t *testing.T) {
	unsetTLSSettings(t)

	tlsConfig := applyTLSSettings(&tls.Config{})
	assert.Zero(t, tlsConfig.MinVersion)
	assert.Zero(t, tlsConfig.MaxVersion)
	assert.Nil(t, tlsConfig.CurvePreferences)
	assert.Nil(t, tlsConfig.CipherSuites)
	assert.Equal(t, []string{"h2", "http/1.1"}, tlsConfig.NextProtos)
}

func TestApplyTLSSettingsConfigured(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.SSLMinimumVersion, "tlsv1.2"))
	t.Cleanup(test.MockVariableValue(&setting.SSLMaximumVersion, "tlsv1.3"))
	t.Cleanup(test.MockVariableValue(&setting.SSLCurvePreferences, []string{"p384"}))
	t.Cleanup(test.MockVariableValue(&setting.SSLCipherSuites, []string{"ecdhe_rsa_with_aes_256_gcm_sha384"}))

	tlsConfig := applyTLSSettings(&tls.Config{})
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
	assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MaxVersion)
	assert.Equal(t, []tls.CurveID{tls.CurveP384}, tlsConfig.CurvePreferences)
	assert.Equal(t, []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384}, tlsConfig.CipherSuites)
}

func TestHTTPSServesTLS13(t *testing.T) {
	unsetTLSSettings(t)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = applyTLSSettings(&tls.Config{})
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	conn, err := tls.Dial("tcp", strings.TrimPrefix(server.URL, "https://"), &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, err)
	defer conn.Close()
	assert.Equal(t, uint16(tls.VersionTLS13), conn.ConnectionState().Version)
}

// TestCurveStringMapCoversGoCurves fails when Go gains a curve the map does not name, so the
// config vocabulary cannot silently go stale the way it did for the post-quantum key exchanges
func TestCurveStringMapCoversGoCurves(t *testing.T) {
	for id := range math.MaxUint16 + 1 {
		name := tls.CurveID(id).String()
		if strings.HasPrefix(name, "CurveID(") {
			continue // crypto/tls has no name for this ID, so it supports no such curve
		}
		assert.Contains(t, curveStringMap, strings.TrimPrefix(strings.ToLower(name), "curve"), "curve %s", name)
	}
}

// TestCipherStringMapCoversLegacyNames pins the names accepted before the map was derived
// from crypto/tls, so no existing SSL_CIPHER_SUITES config silently stops resolving
func TestCipherStringMapCoversLegacyNames(t *testing.T) {
	for _, name := range []string{
		"rsa_with_rc4_128_sha", "rsa_with_3des_ede_cbc_sha", "rsa_with_aes_128_cbc_sha",
		"rsa_with_aes_256_cbc_sha", "rsa_with_aes_128_cbc_sha256", "rsa_with_aes_128_gcm_sha256",
		"rsa_with_aes_256_gcm_sha384", "ecdhe_ecdsa_with_rc4_128_sha", "ecdhe_ecdsa_with_aes_128_cbc_sha",
		"ecdhe_ecdsa_with_aes_256_cbc_sha", "ecdhe_rsa_with_rc4_128_sha", "ecdhe_rsa_with_3des_ede_cbc_sha",
		"ecdhe_rsa_with_aes_128_cbc_sha", "ecdhe_rsa_with_aes_256_cbc_sha", "ecdhe_ecdsa_with_aes_128_cbc_sha256",
		"ecdhe_rsa_with_aes_128_cbc_sha256", "ecdhe_rsa_with_aes_128_gcm_sha256", "ecdhe_ecdsa_with_aes_128_gcm_sha256",
		"ecdhe_rsa_with_aes_256_gcm_sha384", "ecdhe_ecdsa_with_aes_256_gcm_sha384",
		"ecdhe_rsa_with_chacha20_poly1305_sha256", "ecdhe_ecdsa_with_chacha20_poly1305_sha256",
		"ecdhe_rsa_with_chacha20_poly1305", "ecdhe_ecdsa_with_chacha20_poly1305",
		"aes_128_gcm_sha256", "aes_256_gcm_sha384", "chacha20_poly1305_sha256",
	} {
		assert.NotZero(t, cipherStringMap[name], "cipher %q", name)
	}
}
