// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v85/github"
)

// GenerateGitHubAppJWT generates a JWT for GitHub App authentication
func GenerateGitHubAppJWT(clientID string, privateKeyPEM string) (string, error) {
	// Parse the private key
	block, _ := pem.Decode([]byte(strings.TrimSpace(privateKeyPEM)))
	if block == nil {
		return "", errors.New("failed to parse PEM block containing the private key")
	}

	// Try parsing the key as PKCS1, and fall back to PKCS8
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
		Issuer:    clientID,
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

// GetGitHubAppInstallationToken exchanges a JWT for an installation access token.
// It always performs a fresh token exchange — callers should not assume caching.
func GetGitHubAppInstallationToken(ctx context.Context, clientID string, installationID int64, privateKeyPEM, baseURL string, httpTransport *http.Transport) (string, error) {
	// Generate JWT
	jwtToken, err := GenerateGitHubAppJWT(clientID, privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

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
			return "", fmt.Errorf("failed to create GitHub client: %w", err)
		}
	}

	// Get installation access token
	installationToken, _, err := githubClient.Apps.CreateInstallationToken(
		ctx,
		installationID,
		&github.InstallationTokenOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create installation token: %w", err)
	}

	return installationToken.GetToken(), nil
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

	transport := t.transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Error("GitHub App JWT request failed: %v", err)
		return resp, err
	}

	return resp, nil
}
