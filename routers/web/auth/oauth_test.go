// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/session"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"
	"gitea.dev/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createAndParseToken(t *testing.T, grant *auth.OAuth2Grant) *oauth2_provider.OIDCToken {
	signingKey, err := oauth2_provider.CreateJWTSigningKey("HS256", make([]byte, 32))
	assert.NoError(t, err)
	assert.NotNil(t, signingKey)

	response, terr := oauth2_provider.NewAccessTokenResponse(t.Context(), grant, signingKey, signingKey)
	assert.Nil(t, terr)
	assert.NotNil(t, response)
	assert.Equal(t, grant.Scope, response.Scope)

	parsedToken, err := jwt.ParseWithClaims(response.IDToken, &oauth2_provider.OIDCToken{}, func(token *jwt.Token) (any, error) {
		assert.NotNil(t, token.Method)
		assert.Equal(t, signingKey.SigningMethod().Alg(), token.Method.Alg())
		return signingKey.VerifyKey(), nil
	})
	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	oidcToken, ok := parsedToken.Claims.(*oauth2_provider.OIDCToken)
	assert.True(t, ok)
	assert.NotNil(t, oidcToken)

	return oidcToken
}

func TestNewAccessTokenResponse_OIDCToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	grants, err := auth.GetOAuth2GrantsByUserID(t.Context(), 3)
	assert.NoError(t, err)
	assert.Len(t, grants, 1)

	// Scopes: openid
	oidcToken := createAndParseToken(t, grants[0])
	assert.Empty(t, oidcToken.Name)
	assert.Empty(t, oidcToken.PreferredUsername)
	assert.Empty(t, oidcToken.Profile)
	assert.Empty(t, oidcToken.Picture)
	assert.Empty(t, oidcToken.Website)
	assert.Empty(t, oidcToken.UpdatedAt)
	assert.Empty(t, oidcToken.Email)
	assert.False(t, oidcToken.EmailVerified)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	grants, err = auth.GetOAuth2GrantsByUserID(t.Context(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, grants, 1)

	// Scopes: openid profile email
	oidcToken = createAndParseToken(t, grants[0])
	assert.Equal(t, user.DisplayName(), oidcToken.Name)
	assert.Equal(t, user.Name, oidcToken.PreferredUsername)
	assert.Equal(t, user.HTMLURL(t.Context()), oidcToken.Profile)
	assert.Equal(t, user.AvatarLink(t.Context()), oidcToken.Picture)
	assert.Equal(t, user.Website, oidcToken.Website)
	assert.Equal(t, user.UpdatedUnix, oidcToken.UpdatedAt)
	assert.Equal(t, user.Email, oidcToken.Email)
	assert.Equal(t, user.IsActive, oidcToken.EmailVerified)
}

func TestOAuth2AvatarClientBlocksLoopback(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		_, _ = w.Write([]byte("img"))
	}))
	defer srv.Close()

	// the httptest server binds a loopback address, which the SSRF-protected dialer must refuse
	resp, err := oauth2AvatarHTTPClient().Get(srv.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	assert.False(t, hit.Load(), "avatar client must refuse to dial a loopback address")
}

func TestOAuth2AvatarClientBlocksCloudMetadata(t *testing.T) {
	resp, err := oauth2AvatarHTTPClient().Get("http://169.254.169.254/latest/meta-data/")
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	assert.ErrorIs(t, err, policy.ErrDenied,
		"avatar client must refuse a link-local cloud-metadata address")
}

func TestOAuth2ScopeChange(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	app := unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	mockOpt := contexttest.MockContextOption{SessionStore: session.NewMockMemStore("oauth2-scope-change")}
	authorize := func(scope string) *httptest.ResponseRecorder {
		ctx, resp := contexttest.MockContext(t, "/login/oauth/authorize", mockOpt)
		ctx.Doer = doer
		web.SetForm(ctx, &forms.AuthorizationForm{ResponseType: "code", ClientID: app.ClientID, RedirectURI: app.RedirectURIs[0], State: "state", Scope: scope})
		AuthorizeOAuth(ctx)
		return resp
	}
	assert.Equal(t, http.StatusSeeOther, authorize("").Code)
	assert.Equal(t, http.StatusSeeOther, authorize("profile openid").Code)
	grant := unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Grant{ID: 1})
	require.NoError(t, auth.UpdateGrantScope(t.Context(), grant, "openid profile read:user write:user"))
	assert.Equal(t, http.StatusSeeOther, authorize("openid profile write:user").Code)
	require.NoError(t, auth.UpdateGrantScope(t.Context(), grant, "openid invalid:scope"))
	assert.Equal(t, http.StatusOK, authorize("openid profile").Code)
	require.NoError(t, auth.UpdateGrantScope(t.Context(), grant, "openid profile"))
	assert.Equal(t, http.StatusOK, authorize("openid profile email").Code)

	for _, tt := range []struct {
		name, scope, wantScope string
	}{
		{"only invalid scopes", "invalid:scope unsupported", "no-token-scope"},
		{"OIDC with invalid scope", "openid invalid:scope", "openid no-token-scope"},
		{"comma-only scope", ",", "no-token-scope"},
		{"OIDC with comma-only scope", "openid ,,", "openid no-token-scope"},
		{"OIDC scopes", "openid profile email", "openid profile email"},
		{"mixed valid and invalid scopes", "openid groups read:user,invalid:scope public-only unsupported", "openid groups public-only read:user"},
		{"normalized access scopes", " read:user  openid profile email read:user ", "openid profile email read:user"},
		{"redundant read scope", "openid read:user write:user", "openid write:user"},
		{"redundant scope with all", "openid all read:user", "openid all"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp := authorize(tt.scope)
			require.Equal(t, http.StatusOK, resp.Code)
			ctx, resp := contexttest.MockContext(t, "/login/oauth/grant", mockOpt)
			ctx.Doer = doer
			web.SetForm(ctx, &forms.GrantApplicationForm{ClientID: app.ClientID, Granted: true, RedirectURI: app.RedirectURIs[0], State: "state", Scope: tt.scope})
			GrantApplicationOAuth(ctx)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			redirect, err := url.Parse(resp.Header().Get("Location"))
			require.NoError(t, err)
			assert.Empty(t, redirect.Query().Get("error"))
			assert.Equal(t, "state", redirect.Query().Get("state"))
			unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Grant{ID: 1, Scope: tt.wantScope})
			assert.NotEmpty(t, redirect.Query().Get("code"))
			resp = authorize(tt.scope)
			assert.Equal(t, http.StatusSeeOther, resp.Code)
			redirect, err = url.Parse(resp.Header().Get("Location"))
			require.NoError(t, err)
			assert.NotEmpty(t, redirect.Query().Get("code"))
			assert.Empty(t, redirect.Query().Get("error"))
		})
	}
}
