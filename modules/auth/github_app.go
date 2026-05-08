// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v84/github"
)

// GitHubAppTokenCache caches installation access tokens
type GitHubAppTokenCache struct {
	mu     sync.RWMutex
	tokens map[string]*cachedToken
}

type cachedToken struct {
	token      string
	expiresAt  time.Time
	httpClient *http.Client
}

var globalTokenCache = &GitHubAppTokenCache{
	tokens: make(map[string]*cachedToken),
}

// GenerateGitHubAppJWT generates a JWT for GitHub App authentication
func GenerateGitHubAppJWT(appID int64, privateKeyPEM string) (string, error) {
	// Log the private key format for debugging
	lines := strings.Split(privateKeyPEM, "\n")
	if len(lines) > 0 {
		log.Debug("Private key first line: %s", lines[0])
		if len(lines) > 1 {
			log.Debug("Private key last line: %s", lines[len(lines)-1])
		}
		log.Debug("Private key total lines: %d", len(lines))
	}

	// Parse the private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", errors.New("failed to parse PEM block containing the private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return "", errors.New("private key is not RSA")
		}
	}

	// Create JWT claims
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)), // GitHub requires max 10 minutes
		Issuer:    strconv.FormatInt(appID, 10),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Sign token
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signedToken, nil
}

// GetGitHubAppInstallationToken exchanges a JWT for an installation access token
func GetGitHubAppInstallationToken(ctx context.Context, appID, installationID int64, privateKeyPEM, baseURL string, httpTransport *http.Transport) (string, *http.Client, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%d:%d:%s", appID, installationID, baseURL)

	globalTokenCache.mu.RLock()
	cached, exists := globalTokenCache.tokens[cacheKey]
	globalTokenCache.mu.RUnlock()

	if exists && time.Now().Before(cached.expiresAt.Add(-5*time.Minute)) {
		// Token is still valid (with 5 minute buffer)
		return cached.token, cached.httpClient, nil
	}

	// Generate JWT
	jwtToken, err := GenerateGitHubAppJWT(appID, privateKeyPEM)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate JWT: %w", err)
	}
	log.Debug("Generated JWT for GitHub App %d (token: %s...%s)", appID, jwtToken[:20], jwtToken[len(jwtToken)-20:])

	// Create HTTP client with JWT for authentication
	jwtHTTPClient := &http.Client{
		Transport: &jwtTransport{
			token:     jwtToken,
			transport: httpTransport,
		},
	}

	// Create GitHub client
	var githubClient *github.Client
	if baseURL == "" || baseURL == "https://api.github.com" {
		githubClient = github.NewClient(jwtHTTPClient)
	} else {
		githubClient, err = github.NewClient(jwtHTTPClient).WithEnterpriseURLs(baseURL, baseURL)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create GitHub client: %w", err)
		}
	}

	// Get installation access token
	installationToken, _, err := githubClient.Apps.CreateInstallationToken(
		ctx,
		installationID,
		&github.InstallationTokenOptions{},
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create installation token: %w", err)
	}

	// Create HTTP client with installation token
	tokenHTTPClient := &http.Client{
		Transport: &tokenTransport{
			token:     installationToken.GetToken(),
			transport: httpTransport,
		},
	}

	// Cache the token
	globalTokenCache.mu.Lock()
	globalTokenCache.tokens[cacheKey] = &cachedToken{
		token:      installationToken.GetToken(),
		expiresAt:  installationToken.GetExpiresAt().Time,
		httpClient: tokenHTTPClient,
	}
	globalTokenCache.mu.Unlock()

	log.Debug("Generated new GitHub App installation token for app %d, installation %d (expires at %v)",
		appID, installationID, installationToken.GetExpiresAt().Time)

	return installationToken.GetToken(), tokenHTTPClient, nil
}

// jwtTransport is an http.RoundTripper that adds JWT authentication
type jwtTransport struct {
	token     string
	transport http.RoundTripper
}

func (t *jwtTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// GitHub requires "Bearer" prefix for JWT authentication
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	log.Debug("GitHub App JWT request: %s %s", req.Method, req.URL.String())

	transport := t.transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Error("GitHub App JWT request failed: %v", err)
		return resp, err
	}

	log.Debug("GitHub App JWT response: %d %s", resp.StatusCode, resp.Status)
	return resp, nil
}

// tokenTransport is an http.RoundTripper that adds token authentication
type tokenTransport struct {
	token     string
	transport http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "token "+t.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	log.Debug("GitHub App token request: %s %s (token: %s...%s)", req.Method, req.URL.String(), t.token[:20], t.token[len(t.token)-20:])

	transport := t.transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Error("GitHub App token request failed: %v", err)
		return resp, err
	}

	log.Debug("GitHub App token response: %d %s", resp.StatusCode, resp.Status)
	return resp, nil
}

// ClearTokenCache clears the token cache (useful for testing)
func ClearTokenCache() {
	globalTokenCache.mu.Lock()
	defer globalTokenCache.mu.Unlock()
	globalTokenCache.tokens = make(map[string]*cachedToken)
}
