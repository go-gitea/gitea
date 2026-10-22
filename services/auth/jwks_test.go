// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitea.dev/modules/json"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func marshalJWKSet(t *testing.T, keys ...jose.JSONWebKey) []byte {
	data, err := json.Marshal(jose.JSONWebKeySet{Keys: keys})
	require.NoError(t, err)
	return data
}

func TestParseJWKSet(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	data := marshalJWKSet(t,
		jose.JSONWebKey{Key: rsaKey, KeyID: "rsa-private", Use: "sig"}, // private parts must be dropped
		jose.JSONWebKey{Key: &ecKey.PublicKey, KeyID: "ec"},
		jose.JSONWebKey{Key: &ecKey.PublicKey, KeyID: "ec-enc", Use: "enc"},
		jose.JSONWebKey{Key: []byte("symmetric-secret"), KeyID: "hmac"},
	)
	// an unsupported key type must not break the whole set
	data = []byte(strings.Replace(string(data), `"keys":[`, `"keys":[{"kty":"unknown","kid":"bad"},`, 1))

	keys, err := parseJWKSet(data)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	assert.Equal(t, "rsa-private", keys[0].KeyID)
	assert.IsType(t, &rsa.PublicKey{}, keys[0].Key)
	assert.Equal(t, "ec", keys[1].KeyID)

	_, err = parseJWKSet(marshalJWKSet(t, jose.JSONWebKey{Key: []byte("secret")}))
	assert.Error(t, err)
	_, err = parseJWKSet([]byte("not json"))
	assert.Error(t, err)
}

func TestRemoteJWTKeyProvider(t *testing.T) {
	keyA, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	keyB, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	var hits atomic.Int32
	var body atomic.Pointer[[]byte]
	setBody := func(b []byte) { body.Store(&b) }
	var status atomic.Int32
	setBody(marshalJWKSet(t, jose.JSONWebKey{Key: &keyA.PublicKey, KeyID: "a"}))
	status.Store(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write(*body.Load())
	}))
	defer srv.Close()

	p := newRemoteJWTKeyProvider(srv.URL, time.Hour)
	ctx := t.Context()

	assert.Len(t, p.keysFor(ctx, "a", "ES256"), 1)
	assert.Len(t, p.keysFor(ctx, "a", "ES256"), 1)
	assert.EqualValues(t, 1, hits.Load(), "keys should be cached")

	// keys rotated, an unknown kid only refetches once the rate limit allows it
	setBody(marshalJWKSet(t, jose.JSONWebKey{Key: &keyB.PublicKey, KeyID: "b"}))
	assert.Empty(t, p.keysFor(ctx, "b", "ES256"))
	assert.EqualValues(t, 1, hits.Load())
	p.lastAttempt = time.Now().Add(-2 * jwkSetMinRefetchPeriod)
	assert.Len(t, p.keysFor(ctx, "b", "ES256"), 1)
	assert.EqualValues(t, 2, hits.Load())

	// a failed refresh keeps the previous keys
	status.Store(http.StatusInternalServerError)
	p.fetchedAt = time.Now().Add(-2 * time.Hour)
	p.lastAttempt = p.fetchedAt
	assert.Len(t, p.keysFor(ctx, "b", "ES256"), 1)
	assert.EqualValues(t, 3, hits.Load())

	// responses over the size limit are rejected
	status.Store(http.StatusOK)
	setBody([]byte(strings.Repeat(" ", jwkSetMaxSize+1)))
	_, err = p.fetch(ctx)
	assert.ErrorContains(t, err, "too large")
}

func TestPEMFileKeyProvider(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "key.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))
	p, err := newPEMFileKeyProvider(path)
	require.NoError(t, err)
	keys := p.keysFor(t.Context(), "any-kid", "ES256")
	require.Len(t, keys, 1)
	assert.True(t, key.PublicKey.Equal(keys[0]))

	require.NoError(t, os.WriteFile(path, []byte("not pem"), 0o600))
	_, err = newPEMFileKeyProvider(path)
	assert.Error(t, err)
}

func TestMatchJWTKeys(t *testing.T) {
	keys := []jose.JSONWebKey{
		{Key: "k1", KeyID: "1", Algorithm: "RS256"},
		{Key: "k2", KeyID: "2", Algorithm: "ES256"},
		{Key: "k3"},
	}
	assert.Equal(t, []any{"k1", "k3"}, matchJWTKeys(keys, "1", "RS256"))
	assert.Equal(t, []any{"k3"}, matchJWTKeys(keys, "1", "ES256"))
	assert.Equal(t, []any{"k2", "k3"}, matchJWTKeys(keys, "", "ES256"))
}
