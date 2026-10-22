// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"net/http"
	"net/url"

	"gitea.dev/modules/log"
	session_module "gitea.dev/modules/session"

	"golang.org/x/oauth2"
)

func (source *Source) supportsPKCE() bool {
	provider, ok := gothProviders[source.Provider]
	return ok && provider.SupportsPKCE()
}

func pkceSessionKey(sourceName string) string {
	return "oauth2_pkce_verifier:" + sourceName
}

func addPKCEChallengeToURL(rawURL, verifier string) (string, error) {
	authURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := authURL.Query()
	query.Set("code_challenge_method", "S256")
	query.Set("code_challenge", oauth2.S256ChallengeFromVerifier(verifier))
	authURL.RawQuery = query.Encode()
	return authURL.String(), nil
}

func (source *Source) beginPKCE(request *http.Request, authURL string) (string, error) {
	if !source.supportsPKCE() {
		return authURL, nil
	}
	verifier := oauth2.GenerateVerifier()
	sess := session_module.GetContextSession(request)
	if err := sess.Set(pkceSessionKey(source.AuthSource.Name), verifier); err != nil {
		return "", err
	}
	if err := sess.Release(); err != nil {
		return "", err
	}
	return addPKCEChallengeToURL(authURL, verifier)
}

func (source *Source) injectPKCEVerifier(request *http.Request) *http.Request {
	if !source.supportsPKCE() {
		return request
	}
	sess := session_module.GetContextSession(request)
	key := pkceSessionKey(source.AuthSource.Name)
	value := sess.Get(key)
	if value == nil {
		return request
	}
	if verifier, ok := value.(string); ok && verifier != "" {
		request = request.Clone(request.Context()) // Keep the verifier out of access logs of the shared request.
		query := request.URL.Query()
		query.Set("code_verifier", verifier)
		request.URL.RawQuery = query.Encode()
	}
	if err := sess.Delete(key); err != nil {
		log.Error("Failed to delete PKCE verifier from session for source %q: %v", source.AuthSource.Name, err)
	}
	if err := sess.Release(); err != nil {
		log.Error("Failed to release session after consuming PKCE verifier for source %q: %v", source.AuthSource.Name, err)
	}
	return request
}
