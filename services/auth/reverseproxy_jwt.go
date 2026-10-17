// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"

	"github.com/golang-jwt/jwt/v5"
)

// Ensure the struct implements the interface.
var (
	_ Method = &ReverseProxyJWT{}
)

// ReverseProxyJWTMethodName is the constant name of the ReverseProxyJWT authentication method
const ReverseProxyJWTMethodName = "reverse_proxy_jwt"

// ReverseProxyJWT implements the Auth interface for a reverse proxy which passes a signed JWT
// in the "setting.ReverseProxyJWT.Header" header. Unlike ReverseProxy the identity is only
// trusted after the JWT signature and claims have been verified.
type ReverseProxyJWT struct {
	CreateSession bool

	keys *jwtKeyProvider // defaults to the keys loaded from settings
}

// Name represents the name of auth method
func (r *ReverseProxyJWT) Name() string {
	return ReverseProxyJWTMethodName
}

func (r *ReverseProxyJWT) keyProvider() *jwtKeyProvider {
	if r.keys != nil {
		return r.keys
	}
	return reverseProxyJWTKeys
}

// getToken returns the JWT from the configured header, with an optional "Bearer " prefix removed
func (r *ReverseProxyJWT) getToken(req *http.Request) string {
	token := strings.TrimSpace(req.Header.Get(setting.ReverseProxyJWT.Header))
	if len(token) > 7 && strings.EqualFold(token[:7], "Bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return token
}

// parseIdentity verifies the token and extracts the user identity from its claims
func (r *ReverseProxyJWT) parseIdentity(ctx context.Context, token string) (reverseProxyIdentity, error) {
	keys := r.keyProvider()
	if keys == nil {
		return reverseProxyIdentity{}, errors.New("no verification keys configured")
	}
	cfg := setting.ReverseProxyJWT
	opts := []jwt.ParserOption{
		jwt.WithValidMethods(cfg.Algorithms),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(cfg.Leeway),
	}
	if cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.Issuer))
	}
	if cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.Audience))
	}

	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(token, claims, keys.keyfunc(ctx), opts...); err != nil {
		return reverseProxyIdentity{}, err
	}

	id := reverseProxyIdentity{Username: getStringClaim(claims, cfg.UsernameClaim)}
	if id.Username == "" {
		return reverseProxyIdentity{}, errors.New("username claim missing")
	}
	if setting.Service.EnableReverseProxyEmail {
		id.Email = getStringClaim(claims, cfg.EmailClaim)
	}
	if setting.Service.EnableReverseProxyFullName {
		id.FullName = getStringClaim(claims, cfg.FullNameClaim)
	}
	return id, nil
}

// getStringClaim returns the string claim at a dotted path such as "user.login", or "" if it doesn't exist or isn't a string
func getStringClaim(claims map[string]any, path string) string {
	if path == "" {
		return ""
	}
	var cur any = claims
	for part := range strings.SplitSeq(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[part]
	}
	s, _ := cur.(string)
	return strings.TrimSpace(s)
}

// Verify attempts to load a user object based on the JWT sent by the reverse proxy.
// Returns nil if the header is empty. Once a token is present any failure is an error and
// clears the session user, so the request fails instead of falling back to an existing session.
func (r *ReverseProxyJWT) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	token := r.getToken(req)
	if token == "" {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	user, err := r.getUser(req.Context(), token)
	if err != nil {
		if sess != nil {
			_ = sess.Delete(session.KeyUID)
		}
		return nil, err
	}

	if r.CreateSession && sess != nil {
		sessionUID, ok := sess.Get(session.KeyUID).(int64)
		if !ok || sessionUID != user.ID {
			handleSignInNonInteractive(w, req, sess, user)
		}
	}
	store.GetData()["IsReverseProxy"] = true

	log.Trace("ReverseProxyJWT Authorization: Logged in user %-v", user)
	return user, nil
}

func (r *ReverseProxyJWT) getUser(ctx context.Context, token string) (*user_model.User, error) {
	id, err := r.parseIdentity(ctx, token)
	if err != nil {
		log.Debug("ReverseProxyJWT Authorization: invalid token: %v", err)
		return nil, ErrUserAuthMessage("invalid reverse proxy JWT")
	}
	user, err := getReverseProxyUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserAuthMessage("reverse proxy JWT user not found")
	}
	if !user.IsIndividual() {
		return nil, ErrUserAuthMessage("reverse proxy JWT user is not an individual user")
	}
	return user, nil
}
