// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/singleflight"
)

const (
	jwkSetMaxSize          = 1 << 20
	jwkSetFetchTimeout     = 10 * time.Second
	jwkSetMinRefetchPeriod = time.Minute // limits refetches triggered by tokens with an unknown kid
)

// jwtKeyProvider supplies the public keys used to verify reverse proxy JWTs.
// Keys from a URL are cached for ttl and refetched when a token has an unknown kid,
// keys from a file are loaded once.
type jwtKeyProvider struct {
	url    string
	ttl    time.Duration
	client *http.Client
	group  singleflight.Group

	mu          sync.RWMutex
	keys        []jose.JSONWebKey
	fetchedAt   time.Time
	lastAttempt time.Time
}

var reverseProxyJWTKeys *jwtKeyProvider

func initReverseProxyJWTKeys() {
	if !setting.Service.EnableReverseProxyJWTAuth && !setting.Service.EnableReverseProxyJWTAuthAPI {
		return
	}
	var err error
	cfg := setting.ReverseProxyJWT
	switch {
	case cfg.JWKSetURL != "":
		reverseProxyJWTKeys = newRemoteJWTKeyProvider(cfg.JWKSetURL, cfg.CacheTTL)
	case cfg.JWKSetFile != "":
		reverseProxyJWTKeys, err = newJWKSetFileKeyProvider(cfg.JWKSetFile)
	default:
		reverseProxyJWTKeys, err = newPEMFileKeyProvider(cfg.KeyFile)
	}
	if err != nil {
		log.Fatal("Unable to load reverse proxy JWT verification keys: %v", err)
	}
}

func newRemoteJWTKeyProvider(url string, ttl time.Duration) *jwtKeyProvider {
	return &jwtKeyProvider{
		url: url,
		ttl: ttl,
		client: &http.Client{
			Timeout:   jwkSetFetchTimeout,
			Transport: &http.Transport{Proxy: proxy.Proxy()},
		},
	}
}

func newJWKSetFileKeyProvider(path string) (*jwtKeyProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	keys, err := parseJWKSet(data)
	if err != nil {
		return nil, err
	}
	return &jwtKeyProvider{keys: keys}, nil
}

func newPEMFileKeyProvider(path string) (*jwtKeyProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("no PEM data found")
	}
	var pub any
	switch block.Type {
	case "PUBLIC KEY":
		pub, err = x509.ParsePKIXPublicKey(block.Bytes)
	case "CERTIFICATE":
		var cert *x509.Certificate
		if cert, err = x509.ParseCertificate(block.Bytes); err == nil {
			pub = cert.PublicKey
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q, expected PUBLIC KEY or CERTIFICATE", block.Type)
	}
	if err != nil {
		return nil, err
	}
	return &jwtKeyProvider{keys: []jose.JSONWebKey{{Key: pub}}}, nil
}

// parseJWKSet returns the public signing keys of a JWK set. Keys that can't be parsed, symmetric keys
// and keys not meant for signatures are skipped so one unsupported key doesn't break the whole set.
func parseJWKSet(data []byte) ([]jose.JSONWebKey, error) {
	var set struct {
		Keys []json.Value `json:"keys"`
	}
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("invalid JWK set: %w", err)
	}
	keys := make([]jose.JSONWebKey, 0, len(set.Keys))
	for _, raw := range set.Keys {
		var key jose.JSONWebKey
		if err := key.UnmarshalJSON(raw); err != nil {
			log.Warn("Skipping invalid key in reverse proxy JWK set: %v", err)
			continue
		}
		if key.Use != "" && key.Use != "sig" {
			continue
		}
		pub := key.Public() // drops private parts, symmetric keys end up with a nil Key
		if pub.Key == nil {
			continue
		}
		keys = append(keys, pub)
	}
	if len(keys) == 0 {
		return nil, errors.New("JWK set contains no usable signing keys")
	}
	return keys, nil
}

func (p *jwtKeyProvider) fetch(ctx context.Context) ([]jose.JSONWebKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching JWK set", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, jwkSetMaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > jwkSetMaxSize {
		return nil, errors.New("JWK set response too large")
	}
	return parseJWKSet(data)
}

// refresh fetches the remote key set, sharing a single request between concurrent callers.
// On failure the previously fetched keys are kept.
func (p *jwtKeyProvider) refresh(ctx context.Context) {
	_, _, _ = p.group.Do("refresh", func() (any, error) {
		p.mu.Lock()
		p.lastAttempt = time.Now()
		p.mu.Unlock()

		// don't let a cancelled request abort a fetch other requests are waiting for
		keys, err := p.fetch(context.WithoutCancel(ctx))
		if err != nil {
			log.Warn("Unable to fetch reverse proxy JWK set from %s: %v", p.url, err)
			return nil, err
		}
		p.mu.Lock()
		p.keys, p.fetchedAt = keys, time.Now()
		p.mu.Unlock()
		return keys, nil
	})
}

func (p *jwtKeyProvider) currentKeys(ctx context.Context) []jose.JSONWebKey {
	if p.url == "" {
		return p.keys
	}
	p.mu.RLock()
	stale := time.Since(p.fetchedAt) > p.ttl && time.Since(p.lastAttempt) > jwkSetMinRefetchPeriod
	p.mu.RUnlock()
	if stale {
		p.refresh(ctx)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.keys
}

// keysFor returns the keys usable to verify a token with the given kid and alg.
// An unknown kid triggers a rate limited refetch, to pick up keys rotated in by the identity provider.
func (p *jwtKeyProvider) keysFor(ctx context.Context, kid, alg string) []any {
	matches := matchJWTKeys(p.currentKeys(ctx), kid, alg)
	if len(matches) == 0 && kid != "" && p.url != "" {
		p.mu.RLock()
		canRefetch := time.Since(p.lastAttempt) > jwkSetMinRefetchPeriod
		p.mu.RUnlock()
		if canRefetch {
			p.refresh(ctx)
			matches = matchJWTKeys(p.currentKeys(ctx), kid, alg)
		}
	}
	return matches
}

func matchJWTKeys(keys []jose.JSONWebKey, kid, alg string) []any {
	var matches []any
	for _, k := range keys {
		if kid != "" && k.KeyID != "" && k.KeyID != kid {
			continue
		}
		if k.Algorithm != "" && k.Algorithm != alg {
			continue
		}
		matches = append(matches, k.Key)
	}
	return matches
}

// keyfunc returns a jwt.Keyfunc resolving the verification keys for a token.
// Key type mismatches (e.g. an RSA key for an ES256 token) are rejected by the jwt signing methods.
func (p *jwtKeyProvider) keyfunc(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		keys := p.keysFor(ctx, kid, token.Method.Alg())
		switch len(keys) {
		case 0:
			return nil, fmt.Errorf("no verification key found for kid %q", kid)
		case 1:
			return keys[0], nil
		}
		set := jwt.VerificationKeySet{Keys: make([]jwt.VerificationKey, 0, len(keys))}
		for _, k := range keys {
			set.Keys = append(set.Keys, k)
		}
		return set, nil
	}
}
