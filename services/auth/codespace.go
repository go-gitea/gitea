// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"errors"
	"net/http"

	codespace_model "gitea.dev/models/codespace"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/auth/httpauth"
	"gitea.dev/modules/reqctx"
	codespace_service "gitea.dev/services/codespace"
)

// CodespaceTokenMethodName is the constant name of the Codespace Token authentication method.
const CodespaceTokenMethodName = "codespace_token"

// ErrAuthMethodTerminal stops the auth group from trying fallback methods.
var ErrAuthMethodTerminal = errors.New("auth method rejected credential")

// ErrCodespaceTokenForbidden is returned when a valid Codespace Token cannot be used for this request.
var ErrCodespaceTokenForbidden = errors.New("codespace token forbidden")

// IsCodespaceTokenForbidden reports whether an auth error should be returned as authorization failure.
func IsCodespaceTokenForbidden(err error) bool {
	return errors.Is(err, ErrCodespaceTokenForbidden)
}

func codespaceTokenAuthError(err error) error {
	if err == nil || errors.Is(err, codespace_service.ErrResolveGiteaTokenUnmatched) {
		return nil
	}
	if errors.Is(err, codespace_service.ErrResolveGiteaTokenForbidden) {
		return errors.Join(ErrAuthMethodTerminal, ErrCodespaceTokenForbidden)
	}
	return errors.Join(ErrAuthMethodTerminal, err)
}

// CodespaceToken recognizes Codespace Tokens on routes that do not otherwise allow token auth.
type CodespaceToken struct {
	RejectValid bool
}

type (
	codespaceTokenAuthAllowedKey struct{}
)

// SetCodespaceTokenAuthAllowed records whether the current Web route may authenticate Codespace Tokens.
func SetCodespaceTokenAuthAllowed(ctx context.Context, allowed bool) {
	if store := reqctx.GetRequestDataStore(ctx); store != nil {
		store.SetContextValue(codespaceTokenAuthAllowedKey{}, allowed)
	}
}

func codespaceTokenAuthAllowed(ctx context.Context) bool {
	store := reqctx.GetRequestDataStore(ctx)
	return store == nil || store.GetContextValue(codespaceTokenAuthAllowedKey{}) != false
}

func (m *CodespaceToken) Name() string {
	return CodespaceTokenMethodName
}

func (m *CodespaceToken) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	token := parseCodespaceAuthHeaderToken(req)
	if token == "" {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}
	codespaceToken, err := codespace_service.ResolveGiteaToken(req.Context(), token)
	if err != nil {
		return nil, codespaceTokenAuthError(err)
	}
	if m.RejectValid {
		return nil, errors.Join(ErrAuthMethodTerminal, ErrCodespaceTokenForbidden)
	}
	storeCodespaceTokenAuth(store, codespaceToken)
	return codespaceToken.User, nil
}

func parseCodespaceAuthHeaderToken(req *http.Request) string {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parsed, ok := httpauth.ParseAuthorizationHeader(authHeader)
	if !ok {
		return ""
	}
	switch {
	case parsed.BearerToken != nil && codespace_service.IsGiteaTokenCandidate(parsed.BearerToken.Token):
		return parsed.BearerToken.Token
	case parsed.BasicAuth != nil:
		username, password := parsed.BasicAuth.Username, parsed.BasicAuth.Password
		if codespace_service.IsGiteaTokenCandidate(password) {
			return password
		}
		if (password == "" || password == "x-oauth-basic") && codespace_service.IsGiteaTokenCandidate(username) {
			return username
		}
	}
	return ""
}

func storeCodespaceTokenAuth(store DataStore, codespaceToken *codespace_service.GiteaTokenAuthSnapshot) {
	store.GetData()["LoginMethod"] = CodespaceTokenMethodName
	store.GetData()["IsApiToken"] = true
	store.GetData()["ApiTokenScope"] = codespaceToken.Scope
	store.GetData()[codespace_model.GiteaTokenAuthDataKey] = codespaceToken
}

// CodespaceTokenSnapshot returns the Codespace Token auth snapshot stored on the request.
func CodespaceTokenSnapshot(store DataStore) (*codespace_service.GiteaTokenAuthSnapshot, bool) {
	snapshot, ok := store.GetData()[codespace_model.GiteaTokenAuthDataKey].(*codespace_service.GiteaTokenAuthSnapshot)
	return snapshot, ok && snapshot != nil
}

var _ Method = &CodespaceToken{}
