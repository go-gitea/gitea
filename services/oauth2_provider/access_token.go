// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_provider //nolint

import (
	"context"
	"fmt"
	"slices"
	"strings"

	auth "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/golang-jwt/jwt/v5"
)

// AccessTokenErrorCode represents an error code specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
type AccessTokenErrorCode string

const (
	// AccessTokenErrorCodeInvalidRequest represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidRequest AccessTokenErrorCode = "invalid_request"
	// AccessTokenErrorCodeInvalidClient represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidClient = "invalid_client"
	// AccessTokenErrorCodeInvalidGrant represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidGrant = "invalid_grant"
	// AccessTokenErrorCodeUnauthorizedClient represents an error code specified in RFC 6749
	AccessTokenErrorCodeUnauthorizedClient = "unauthorized_client"
	// AccessTokenErrorCodeUnsupportedGrantType represents an error code specified in RFC 6749
	AccessTokenErrorCodeUnsupportedGrantType = "unsupported_grant_type"
	// AccessTokenErrorCodeInvalidScope represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidScope = "invalid_scope"
)

// AccessTokenError represents an error response specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
type AccessTokenError struct {
	ErrorCode        AccessTokenErrorCode `json:"error" form:"error"`
	ErrorDescription string               `json:"error_description"`
}

// Error returns the error message
func (err AccessTokenError) Error() string {
	return fmt.Sprintf("%s: %s", err.ErrorCode, err.ErrorDescription)
}

// TokenType specifies the kind of token
type TokenType string

const (
	// TokenTypeBearer represents a token type specified in RFC 6749
	TokenTypeBearer TokenType = "bearer"
	// TokenTypeMAC represents a token type specified in RFC 6749
	TokenTypeMAC = "mac"
)

// AccessTokenResponse represents a successful access token response
// https://datatracker.ietf.org/doc/html/rfc6749#section-4.2.2
type AccessTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    TokenType `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token,omitempty"`
}

// GrantAdditionalScopes returns valid scopes coming from grant
func GrantAdditionalScopes(grantScopes string) auth.AccessTokenScope {
	// scopes_supported from templates/user/auth/oidc_wellknown.tmpl
	generalScopesSupported := []string{
		"openid",
		"profile",
		"email",
		"groups",
	}

	var accessScopes []string // the scopes for access control, but not for general information
	for _, scope := range strings.Split(grantScopes, " ") {
		if scope != "" && !slices.Contains(generalScopesSupported, scope) {
			accessScopes = append(accessScopes, scope)
		}
	}

	// since version 1.22, access tokens grant full access to the API
	// with this access is reduced only if additional scopes are provided
	if len(accessScopes) > 0 {
		accessTokenScope := auth.AccessTokenScope(strings.Join(accessScopes, ","))
		if normalizedAccessTokenScope, err := accessTokenScope.Normalize(); err == nil {
			return normalizedAccessTokenScope
		}
		// TODO: if there are invalid access scopes (err != nil),
		// then it is treated as "all", maybe in the future we should make it stricter to return an error
		// at the moment, to avoid breaking 1.22 behavior, invalid tokens are also treated as "all"
	}
	// fallback, empty access scope is treated as "all" access
	return auth.AccessTokenScopeAll
}

func NewAccessTokenResponse(ctx context.Context, grant *auth.OAuth2Grant, serverKey, clientKey JWTSigningKey) (*AccessTokenResponse, *AccessTokenError) {
	if setting.OAuth2.InvalidateRefreshTokens {
		if err := grant.IncreaseCounter(ctx); err != nil {
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidGrant,
				ErrorDescription: "cannot increase the grant counter",
			}
		}
	}
	// generate access token to access the API
	expirationDate := timeutil.TimeStampNow().Add(setting.OAuth2.AccessTokenExpirationTime)
	accessToken := &Token{
		GrantID: grant.ID,
		Kind:    KindAccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationDate.AsTime()),
		},
	}
	signedAccessToken, err := accessToken.SignToken(serverKey)
	if err != nil {
		return nil, &AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot sign token",
		}
	}

	// generate refresh token to request an access token after it expired later
	refreshExpirationDate := timeutil.TimeStampNow().Add(setting.OAuth2.RefreshTokenExpirationTime * 60 * 60).AsTime()
	refreshToken := &Token{
		GrantID: grant.ID,
		Counter: grant.Counter,
		Kind:    KindRefreshToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpirationDate),
		},
	}
	signedRefreshToken, err := refreshToken.SignToken(serverKey)
	if err != nil {
		return nil, &AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot sign token",
		}
	}

	// generate OpenID Connect id_token
	signedIDToken := ""
	if grant.ScopeContains("openid") {
		app, err := auth.GetOAuth2ApplicationByID(ctx, grant.ApplicationID)
		if err != nil {
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "cannot find application",
			}
		}
		user, err := user_model.GetUserByID(ctx, grant.UserID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				return nil, &AccessTokenError{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "cannot find user",
				}
			}
			log.Error("Error loading user: %v", err)
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "server error",
			}
		}

		idToken := &OIDCToken{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationDate.AsTime()),
				Issuer:    setting.AppURL,
				Audience:  []string{app.ClientID},
				Subject:   fmt.Sprint(grant.UserID),
			},
			Nonce: grant.Nonce,
		}
		if grant.ScopeContains("profile") {
			idToken.Name = user.DisplayName()
			idToken.PreferredUsername = user.Name
			idToken.Profile = user.HTMLURL()
			idToken.Picture = user.AvatarLink(ctx)
			idToken.Website = user.Website
			idToken.Locale = user.Language
			idToken.UpdatedAt = user.UpdatedUnix
		}
		if grant.ScopeContains("email") {
			idToken.Email = user.Email
			idToken.EmailVerified = user.IsActive
		}
		if grant.ScopeContains("groups") {
			accessTokenScope := GrantAdditionalScopes(grant.Scope)

			// since version 1.22 does not verify if groups should be public-only,
			// onlyPublicGroups will be set only if 'public-only' is included in a valid scope
			onlyPublicGroups, _ := accessTokenScope.PublicOnly()

			groups, err := GetOAuthGroupsForUser(ctx, user, onlyPublicGroups)
			if err != nil {
				log.Error("Error getting groups: %v", err)
				return nil, &AccessTokenError{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "server error",
				}
			}
			idToken.Groups = groups
		}

		signedIDToken, err = idToken.SignToken(clientKey)
		if err != nil {
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "cannot sign token",
			}
		}
	}

	return &AccessTokenResponse{
		AccessToken:  signedAccessToken,
		TokenType:    TokenTypeBearer,
		ExpiresIn:    setting.OAuth2.AccessTokenExpirationTime,
		RefreshToken: signedRefreshToken,
		IDToken:      signedIDToken,
	}, nil
}

// returns a list of "org" and "org:team" strings,
// that the given user is a part of.
func GetOAuthGroupsForUser(ctx context.Context, user *user_model.User, onlyPublicGroups bool) ([]string, error) {
	orgs, err := db.Find[org_model.Organization](ctx, org_model.FindOrgOptions{
		UserID:         user.ID,
		IncludePrivate: !onlyPublicGroups,
	})
	if err != nil {
		return nil, fmt.Errorf("GetUserOrgList: %w", err)
	}

	orgTeams, err := org_model.OrgList(orgs).LoadTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("LoadTeams: %w", err)
	}

	var groups []string
	for _, org := range orgs {
		groups = append(groups, org.Name)
		for _, team := range orgTeams[org.ID] {
			if team.IsMember(ctx, user.ID) {
				groups = append(groups, org.Name+":"+team.LowerName)
			}
		}
	}
	return groups, nil
}
