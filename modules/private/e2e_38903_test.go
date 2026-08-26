// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selfSignedCertFor generates an in-memory self-signed cert/key pair valid only for the
// given DNS name -- like a real ACME/CA-issued cert, it can never carry the 0.0.0.0
// wildcard bind address as a SAN.
func selfSignedCertFor(t *testing.T, dnsName string) tls.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{dnsName},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)

	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	)
	require.NoError(t, err)
	return cert
}

// startInternalAPITestServer starts a real TLS listener bound to 0.0.0.0 (mirroring the
// HTTP_ADDR default), presenting a cert valid only for "sub.example.com" -- like a real
// ACME/CA-issued cert, which can never carry the 0.0.0.0 wildcard bind address as a SAN.
func startInternalAPITestServer(t *testing.T) (port int) {
	t.Helper()
	setting.InternalToken = "test-token"
	t.Cleanup(func() { setting.InternalToken = "" })

	cert := selfSignedCertFor(t, "sub.example.com")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/internal/serv/none/50", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"key":null,"user":null}`)
	})

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	port = ln.Addr().(*net.TCPAddr).Port

	srv := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{Handler: mux},
		TLS:      &tls.Config{Certificates: []tls.Certificate{cert}},
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return port
}

// requestWithFreshTransport issues the same request NewInternalRequest would, but builds a
// transport from scratch instead of going through the process-wide internalAPITransport
// sync.OnceValue cache. That singleton bakes in whatever setting.LocalURL/Protocol it saw on
// its first call in this test binary, which would make these two scenarios order-dependent;
// a real `gitea serv` invocation always gets a fresh process, so this mirrors that instead.
func requestWithFreshTransport(t *testing.T, url string) ResponseExtra {
	t.Helper()
	transport := &http.Transport{
		DialContext: dialContextInternalAPI,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: internalAPIConnectionIsLocal(setting.Protocol, setting.LocalURL), //nolint:gosec // exercising the exact production decision under test
		},
	}
	req := NewInternalRequest(t.Context(), url, "GET").SetTransport(transport)
	_, extra := requestJSONResp(req, &KeyAndOwner{})
	return extra
}

// TestE2E_Issue38903_BeforeFix reproduces https://github.com/go-gitea/gitea/issues/38903
// verbatim: setting.LocalURL literally "https://0.0.0.0:<port>/" -- exactly what
// LOCAL_ROOT_URL = %(PROTOCOL)s://%(HTTP_ADDR)s:%(HTTP_PORT)s/ produced (before this fix)
// when HTTP_ADDR was left at its default, per the issue's app.ini. A real internal API
// request (the same helper ServNoCommand uses) must fail exactly like the issue reports
// ("Internal Server Connection Error", TLS failure against the literal 0.0.0.0 host).
func TestE2E_Issue38903_BeforeFix(t *testing.T) {
	port := startInternalAPITestServer(t)
	setting.LocalURL = fmt.Sprintf("https://0.0.0.0:%d/", port)
	setting.Protocol = setting.HTTPS

	extra := requestWithFreshTransport(t, setting.LocalURL+"api/internal/serv/none/50")
	require.True(t, extra.HasError())
	assert.Equal(t, "Internal Server Connection Error", extra.UserMsg)
	t.Logf("reproduced issue error: %v", extra.Error)
}

// TestE2E_Issue38903_AfterFix proves the fix: setting.LocalURL "https://localhost:<port>/"
// -- what loadServerFrom now produces for the same app.ini after the modules/setting/server.go
// change -- succeeds against the same TLS server that failed above.
func TestE2E_Issue38903_AfterFix(t *testing.T) {
	port := startInternalAPITestServer(t)
	setting.LocalURL = fmt.Sprintf("https://localhost:%d/", port)
	setting.Protocol = setting.HTTPS

	extra := requestWithFreshTransport(t, setting.LocalURL+"api/internal/serv/none/50")
	require.False(t, extra.HasError(), "expected success, got: %v", extra.Error)
}
