// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateGitHubAppJWT_ValidPKCS1Key(t *testing.T) {
	// Generate a real RSA key pair at test time
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Encode it as PKCS1 PEM
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})

	// Test JWT generation
	token, err := GenerateGitHubAppJWT("Iv1.test123", string(pemBlock))
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// JWT should have 3 dot-separated parts (header.payload.signature)
	parts := strings.Split(token, ".")
	assert.Len(t, parts, 3, "JWT should have 3 dot-separated parts")
}

func TestGenerateGitHubAppJWT_ValidPKCS8Key(t *testing.T) {
	// Generate a real RSA key pair at test time
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Encode it as PKCS8 PEM
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	// Test JWT generation
	token, err := GenerateGitHubAppJWT("Iv1.test456", string(pemBlock))
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// JWT should have 3 dot-separated parts (header.payload.signature)
	parts := strings.Split(token, ".")
	assert.Len(t, parts, 3, "JWT should have 3 dot-separated parts")
}

func TestGenerateGitHubAppJWT_NonRSAKey(t *testing.T) {
	// Generate an EC key (not RSA) to test the "private key is not RSA" error path
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// Encode it as PKCS8 PEM (EC keys can't be PKCS1)
	keyBytes, err := x509.MarshalPKCS8PrivateKey(ecKey)
	require.NoError(t, err)

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	// Test JWT generation - should fail because it's not RSA
	token, err := GenerateGitHubAppJWT("Iv1.test789", string(pemBlock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private key is not RSA")
	assert.Empty(t, token)
}

func TestGenerateGitHubAppJWT_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		clientID    string
		privateKey  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Empty private key",
			clientID:    "123456",
			privateKey:  "",
			expectError: true,
			errorMsg:    "failed to parse PEM block",
		},
		{
			name:        "Invalid PEM format",
			clientID:    "123456",
			privateKey:  "not a valid pem key",
			expectError: true,
			errorMsg:    "failed to parse PEM block",
		},
		{
			name:        "Malformed PEM block",
			clientID:    "123456",
			privateKey:  "-----BEGIN RSA PRIVATE KEY-----\ninvalid\n-----END RSA PRIVATE KEY-----",
			expectError: true,
			errorMsg:    "failed to parse PEM block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateGitHubAppJWT(tt.clientID, tt.privateKey)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, token)
			}
		})
	}
}

func TestGenerateGitHubAppJWT_WhitespacePrivateKey(t *testing.T) {
	// Generate a real RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create PKCS1 PEM
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}))

	// Create PKCS8 PEM
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	pkcs8PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}))

	tests := []struct {
		name       string
		privateKey string
	}{
		{"PKCS1 with leading spaces", "   " + pkcs1PEM},
		{"PKCS1 with trailing spaces", pkcs1PEM + "   "},
		{"PKCS1 with leading newlines", "\n\n\n" + pkcs1PEM},
		{"PKCS1 with trailing newlines", pkcs1PEM + "\n\n\n"},
		{"PKCS1 with leading and trailing whitespace", "  \n\t " + pkcs1PEM + " \t\n  "},
		{"PKCS8 with leading spaces", "   " + pkcs8PEM},
		{"PKCS8 with trailing spaces", pkcs8PEM + "   "},
		{"PKCS8 with leading newlines", "\n\n\n" + pkcs8PEM},
		{"PKCS8 with trailing newlines", pkcs8PEM + "\n\n\n"},
		{"PKCS8 with leading and trailing whitespace", "  \n\t " + pkcs8PEM + " \t\n  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateGitHubAppJWT("Iv1.whitespace", tt.privateKey)
			require.NoError(t, err)
			assert.NotEmpty(t, token)

			parts := strings.Split(token, ".")
			assert.Len(t, parts, 3, "JWT should have 3 dot-separated parts")
		})
	}
}

func TestGetGitHubAppInstallationToken(t *testing.T) {
	// Generate a real RSA key for the mock server
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})

	callCount := 0

	// Create a mock GitHub API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's the installation token endpoint
		// go-github uses /api/v3 prefix for enterprise URLs
		if (r.URL.Path == "/api/v3/app/installations/12345/access_tokens" ||
			r.URL.Path == "/app/installations/12345/access_tokens") && r.Method == http.MethodPost {
			callCount++
			// Return a mock installation token response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			expiresAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
			fmt.Fprintf(w, `{"token":"ghs_mock_token_%d","expires_at":"%s"}`, callCount, expiresAt)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Get the mock server's transport for proper test server communication
	mockTransport := mockServer.Client().Transport.(*http.Transport)

	// Test 1: Should always call API (no caching)
	token1, err := GetGitHubAppInstallationToken(
		context.Background(),
		"Iv1.test123",
		12345,
		string(pemBlock),
		mockServer.URL,
		mockTransport,
	)
	require.NoError(t, err)
	assert.Equal(t, "ghs_mock_token_1", token1)
	assert.Equal(t, 1, callCount)

	// Test 2: Should call API again (no caching)
	token2, err := GetGitHubAppInstallationToken(
		context.Background(),
		"Iv1.test123",
		12345,
		string(pemBlock),
		mockServer.URL,
		mockTransport,
	)
	require.NoError(t, err)
	assert.Equal(t, "ghs_mock_token_2", token2)
	assert.Equal(t, 2, callCount, "Should have made a second API call (no caching)")
}

func TestGetGitHubAppInstallationToken_InvalidKey(t *testing.T) {
	// Test with invalid private key
	_, err := GetGitHubAppInstallationToken(
		context.Background(),
		"Iv1.test123",
		12345,
		"not-a-valid-key",
		"https://api.github.com",
		http.DefaultTransport.(*http.Transport),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate JWT")
}

func TestJWTTransport_SetsHeaders(t *testing.T) {
	const testToken = "jwt-test-token-12345"

	var capturedHeaders http.Header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	transport := &jwtTransport{
		token:     testToken,
		transport: mockServer.Client().Transport,
	}

	req, err := http.NewRequest(http.MethodGet, mockServer.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Bearer "+testToken, capturedHeaders.Get("Authorization"))
	assert.Equal(t, "application/vnd.github+json", capturedHeaders.Get("Accept"))
	assert.Equal(t, "2022-11-28", capturedHeaders.Get("X-GitHub-Api-Version"))
}

func TestJWTTransport_NilTransportFallsBackToDefault(t *testing.T) {
	// When transport is nil, jwtTransport should fall back to http.DefaultTransport
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fallback-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	transport := &jwtTransport{
		token:     "fallback-token",
		transport: nil, // nil transport — should fall back
	}

	req, err := http.NewRequest(http.MethodGet, mockServer.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestJWTTransport_PropagatesTransportError(t *testing.T) {
	// Create a server and immediately close it to get a connection error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serverURL := mockServer.URL
	mockServer.Close() // close immediately so connections will fail

	transport := &jwtTransport{
		token:     "error-test-token",
		transport: &http.Transport{},
	}

	req, err := http.NewRequest(http.MethodGet, serverURL+"/test", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.Error(t, err, "should propagate transport error when server is unreachable")
	if resp != nil {
		defer resp.Body.Close()
	}
}

func TestJWTTransport_DoesNotOverwriteOtherHeaders(t *testing.T) {
	var capturedHeaders http.Header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	transport := &jwtTransport{
		token:     "header-test-token",
		transport: mockServer.Client().Transport,
	}

	req, err := http.NewRequest(http.MethodGet, mockServer.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("X-Custom-Header", "custom-value")

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Custom header should be preserved
	assert.Equal(t, "custom-value", capturedHeaders.Get("X-Custom-Header"))
	// Auth headers should still be set
	assert.Equal(t, "Bearer header-test-token", capturedHeaders.Get("Authorization"))
}
