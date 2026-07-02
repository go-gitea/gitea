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

// supportsPKCE reports whether the source's provider type opts into PKCE (see Provider.SupportsPKCE).
// PKCE is gated on the provider capability rather than a hardcoded provider name so it stays correct as
// OIDC-based providers (e.g. AWS Cognito, which embeds OpenIDProvider) are added.
func (source *Source) supportsPKCE() bool {
	p, ok := gothProviders[source.Provider]
	return ok && p.SupportsPKCE()
}

// pkceSessionKey returns the Gitea-session key under which the PKCE code_verifier is stored for a login source.
// Keyed per source name (unique) to avoid collision with other session keys.
func pkceSessionKey(sourceName string) string {
	return "oauth2_pkce_verifier:" + sourceName
}

// addPKCEChallengeToURL parses rawURL (the gothic authorization redirect URL) and appends an S256 code_challenge
// derived from verifier, preserving all existing query params (client_id, redirect_uri, state, scope, ...).
// Returns the augmented URL.
func addPKCEChallengeToURL(rawURL, verifier string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("code_challenge_method", "S256")
	q.Set("code_challenge", oauth2.S256ChallengeFromVerifier(verifier))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// beginPKCE generates a PKCE code_verifier for a PKCE-capable login source, stashes it in the Gitea session,
// and returns authURL with the matching S256 code_challenge appended so IdPs that REQUIRE PKCE accept the
// login (gitea#21376, gitea#34747). For any other provider authURL is returned unchanged.
//
// goth's provider-level authCodeOptions are shared/static and cannot carry a per-request verifier, hence the
// manual challenge plus own-session stash instead of going through goth. The verifier is keyed by source name
// (like gothic's own _gothic_session), so concurrent logins to the same source in one browser session share a
// slot; this matches gothic's existing single-flight limitation.
//
// Must be called AFTER gothic.GetAuthURL: GetAuthURL triggers RegenerateSession, which carries data forward,
// so storing the verifier afterwards lands it in the surviving session.
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

// injectPKCEVerifier writes the stashed PKCE code_verifier for an OIDC login source into the callback request's
// query so goth's openidConnect Session.Authorize (which reads params.Get("code_verifier")) forwards it to the
// token Exchange. It is a no-op for any other provider or when no verifier was stored.
//
// gothic.CompleteUserAuth builds its params from req.URL.Query() for the GET callback route, so the RawQuery
// rewrite here is how the verifier reaches the exchange. If a goth upgrade changes where it reads the verifier
// from, TestInjectPKCEVerifier still passes but real logins break, so re-verify that seam on any goth bump.
func (source *Source) injectPKCEVerifier(request *http.Request) {
	if !source.supportsPKCE() {
		return
	}
	sess := session_module.GetContextSession(request)
	key := pkceSessionKey(source.AuthSource.Name)
	v := sess.Get(key)
	if v == nil {
		return
	}
	if verifier, ok := v.(string); ok && verifier != "" {
		q := request.URL.Query()
		q.Set("code_verifier", verifier)
		request.URL.RawQuery = q.Encode()
	}
	// Best-effort single-use cleanup: the verifier is already injected, so a failure here must not
	// break the login, but it shouldn't be silent either (a stale verifier would otherwise be invisible).
	if err := sess.Delete(key); err != nil {
		log.Error("Failed to delete PKCE verifier from session for source %q: %v", source.AuthSource.Name, err)
	}
	if err := sess.Release(); err != nil {
		log.Error("Failed to release session after consuming PKCE verifier for source %q: %v", source.AuthSource.Name, err)
	}
}
