// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	oauth2_provider "code.gitea.io/gitea/services/oauth2_provider"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeNoClientID(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize")
	ctx := loginUser(t, "user2")
	resp := ctx.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Client ID not registered")
}

func TestAuthorizeUnregisteredRedirect(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=UNREGISTERED&response_type=code&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Unregistered Redirect URI")
}

func TestAuthorizeUnsupportedResponseType(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=a&response_type=UNEXPECTED&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "unsupported_response_type", u.Query().Get("error"))
	assert.Equal(t, "Only code response type is supported.", u.Query().Get("error_description"))
}

func TestAuthorizeUnsupportedCodeChallengeMethod(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=a&response_type=code&state=thestate&code_challenge_method=UNEXPECTED")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", u.Query().Get("error"))
	assert.Equal(t, "unsupported code challenge method", u.Query().Get("error_description"))
}

func TestAuthorizeLoginRedirect(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize")
	assert.Contains(t, MakeRequest(t, req, http.StatusSeeOther).Body.String(), "/user/login")
}

func TestAuthorizeShow(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=a&response_type=code&state=thestate")
	ctx := loginUser(t, "user4")
	resp := ctx.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	AssertHTMLElement(t, htmlDoc, "#authorize-app", true)
	htmlDoc.GetCSRF()
}

func TestAuthorizeRedirectWithExistingGrant(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=https%3A%2F%2Fexample.com%2Fxyzzy&response_type=code&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "thestate", u.Query().Get("state"))
	assert.Greaterf(t, len(u.Query().Get("code")), 30, "authorization code '%s' should be longer then 30", u.Query().Get("code"))
	u.RawQuery = ""
	assert.Equal(t, "https://example.com/xyzzy", u.String())
}

func TestAuthorizePKCERequiredForPublicClient(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=ce5a1322-42a7-11ed-b878-0242ac120002&redirect_uri=http%3A%2F%2F127.0.0.1&response_type=code&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", u.Query().Get("error"))
	assert.Equal(t, "PKCE is required for public clients", u.Query().Get("error_description"))
}

func TestAccessTokenExchange(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp := MakeRequest(t, req, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsed))
	assert.Greater(t, len(parsed.AccessToken), 10)
	assert.Greater(t, len(parsed.RefreshToken), 10)
}

func TestAccessTokenExchangeWithPublicClient(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "ce5a1322-42a7-11ed-b878-0242ac120002",
		"redirect_uri":  "http://127.0.0.1",
		"code":          "authcodepublic",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp := MakeRequest(t, req, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsed))
	assert.Greater(t, len(parsed.AccessToken), 10)
	assert.Greater(t, len(parsed.RefreshToken), 10)
}

func TestAccessTokenExchangeJSON(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithJSON(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp := MakeRequest(t, req, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsed))
	assert.Greater(t, len(parsed.AccessToken), 10)
	assert.Greater(t, len(parsed.RefreshToken), 10)
}

func TestAccessTokenExchangeWithoutPKCE(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "authcode",
	})
	resp := MakeRequest(t, req, http.StatusBadRequest)
	parsedError := new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "failed PKCE code challenge", parsedError.ErrorDescription)
}

func TestAccessTokenExchangeWithInvalidCredentials(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// invalid client id
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "???",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp := MakeRequest(t, req, http.StatusBadRequest)
	parsedError := new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "invalid_client", string(parsedError.ErrorCode))
	assert.Equal(t, "cannot load client with client id: '???'", parsedError.ErrorDescription)

	// invalid client secret
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "???",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "invalid client secret", parsedError.ErrorDescription)

	// invalid redirect uri
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "???",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "unexpected redirect URI", parsedError.ErrorDescription)

	// invalid authorization code
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "???",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "client is not authorized", parsedError.ErrorDescription)

	// invalid grant_type
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "???",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unsupported_grant_type", string(parsedError.ErrorCode))
	assert.Equal(t, "Only refresh_token or authorization_code grant type is supported", parsedError.ErrorDescription)
}

func TestAccessTokenExchangeWithBasicAuth(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OjRNSzhOYTZSNTVzbWRDWTBXdUNDdW1aNmhqUlBuR1k1c2FXVlJISGpKaUE9")
	resp := MakeRequest(t, req, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsed))
	assert.Greater(t, len(parsed.AccessToken), 10)
	assert.Greater(t, len(parsed.RefreshToken), 10)

	// use wrong client_secret
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OmJsYWJsYQ==")
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError := new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "invalid client secret", parsedError.ErrorDescription)

	// missing header
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "invalid_client", string(parsedError.ErrorCode))
	assert.Equal(t, "cannot load client with client id: ''", parsedError.ErrorDescription)

	// client_id inconsistent with Authorization header
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":   "authorization_code",
		"redirect_uri": "a",
		"code":         "authcode",
		"client_id":    "inconsistent",
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OjRNSzhOYTZSNTVzbWRDWTBXdUNDdW1aNmhqUlBuR1k1c2FXVlJISGpKaUE9")
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "invalid_request", string(parsedError.ErrorCode))
	assert.Equal(t, "client_id in request body inconsistent with Authorization header", parsedError.ErrorDescription)

	// client_secret inconsistent with Authorization header
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"redirect_uri":  "a",
		"code":          "authcode",
		"client_secret": "inconsistent",
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OjRNSzhOYTZSNTVzbWRDWTBXdUNDdW1aNmhqUlBuR1k1c2FXVlJISGpKaUE9")
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "invalid_request", string(parsedError.ErrorCode))
	assert.Equal(t, "client_secret in request body inconsistent with Authorization header", parsedError.ErrorDescription)
}

func TestRefreshTokenInvalidation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp := MakeRequest(t, req, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsed))

	// test without invalidation
	setting.OAuth2.InvalidateRefreshTokens = false

	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type": "refresh_token",
		"client_id":  "da7da3ba-9a13-4167-856f-3899de0b0138",
		// omit secret
		"redirect_uri":  "a",
		"refresh_token": parsed.RefreshToken,
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError := new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "invalid_client", string(parsedError.ErrorCode))
	assert.Equal(t, "invalid empty client secret", parsedError.ErrorDescription)

	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"refresh_token": "UNEXPECTED",
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "unable to parse refresh token", parsedError.ErrorDescription)

	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"refresh_token": parsed.RefreshToken,
	})

	bs, err := io.ReadAll(req.Body)
	assert.NoError(t, err)

	req.Body = io.NopCloser(bytes.NewReader(bs))
	MakeRequest(t, req, http.StatusOK)

	req.Body = io.NopCloser(bytes.NewReader(bs))
	MakeRequest(t, req, http.StatusOK)

	// test with invalidation
	setting.OAuth2.InvalidateRefreshTokens = true
	req.Body = io.NopCloser(bytes.NewReader(bs))
	MakeRequest(t, req, http.StatusOK)

	// repeat request should fail
	req.Body = io.NopCloser(bytes.NewReader(bs))
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "token was already used", parsedError.ErrorDescription)
}

func TestOAuthIntrospection(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp := MakeRequest(t, req, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsed))
	assert.Greater(t, len(parsed.AccessToken), 10)
	assert.Greater(t, len(parsed.RefreshToken), 10)

	// successful request with a valid client_id/client_secret and a valid token
	req = NewRequestWithValues(t, "POST", "/login/oauth/introspect", map[string]string{
		"token": parsed.AccessToken,
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OjRNSzhOYTZSNTVzbWRDWTBXdUNDdW1aNmhqUlBuR1k1c2FXVlJISGpKaUE9")
	resp = MakeRequest(t, req, http.StatusOK)
	type introspectResponse struct {
		Active   bool   `json:"active"`
		Scope    string `json:"scope,omitempty"`
		Username string `json:"username"`
	}
	introspectParsed := new(introspectResponse)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), introspectParsed))
	assert.True(t, introspectParsed.Active)
	assert.Equal(t, "user1", introspectParsed.Username)

	// successful request with a valid client_id/client_secret, but an invalid token
	req = NewRequestWithValues(t, "POST", "/login/oauth/introspect", map[string]string{
		"token": "xyzzy",
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OjRNSzhOYTZSNTVzbWRDWTBXdUNDdW1aNmhqUlBuR1k1c2FXVlJISGpKaUE9")
	resp = MakeRequest(t, req, http.StatusOK)
	introspectParsed = new(introspectResponse)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), introspectParsed))
	assert.False(t, introspectParsed.Active)

	// unsuccessful request with an invalid client_id/client_secret
	req = NewRequestWithValues(t, "POST", "/login/oauth/introspect", map[string]string{
		"token": parsed.AccessToken,
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OjRNSzhOYTZSNTVzbWRDWTBXdUNDdW1aNmhqUlBuR1k1c2FXVlJISGpK")
	resp = MakeRequest(t, req, http.StatusUnauthorized)
	assert.Contains(t, resp.Body.String(), "no valid authorization")
}

func TestOAuth_GrantScopesReadUserFailRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"a",
		},
		ConfidentialClient: true,
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	var app *api.OAuth2Application
	DecodeJSON(t, resp, &app)

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid read:user",
	}

	err := db.Insert(db.DefaultContext, grant)
	require.NoError(t, err)

	assert.Contains(t, grant.Scope, "openid read:user")

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=a&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]

	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "a",
		"code":          authcode,
	})
	accessTokenResp := ctx.MakeRequest(t, accessTokenReq, 200)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	require.NoError(t, json.Unmarshal(accessTokenResp.Body.Bytes(), parsed))
	userReq := NewRequest(t, "GET", "/api/v1/user")
	userReq.SetHeader("Authorization", "Bearer "+parsed.AccessToken)
	userResp := MakeRequest(t, userReq, http.StatusOK)

	type userResponse struct {
		Login string `json:"login"`
		Email string `json:"email"`
	}

	userParsed := new(userResponse)
	require.NoError(t, json.Unmarshal(userResp.Body.Bytes(), userParsed))
	assert.Contains(t, userParsed.Email, "user2@example.com")

	errorReq := NewRequest(t, "GET", "/api/v1/users/user2/repos")
	errorReq.SetHeader("Authorization", "Bearer "+parsed.AccessToken)
	errorResp := MakeRequest(t, errorReq, http.StatusForbidden)

	type errorResponse struct {
		Message string `json:"message"`
	}

	errorParsed := new(errorResponse)
	require.NoError(t, json.Unmarshal(errorResp.Body.Bytes(), errorParsed))
	assert.Contains(t, errorParsed.Message, "token does not have at least one of required scope(s), required=[read:repository]")
}

func TestOAuth_GrantScopesReadRepositoryFailOrganization(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"a",
		},
		ConfidentialClient: true,
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	var app *api.OAuth2Application
	DecodeJSON(t, resp, &app)

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid read:user read:repository",
	}

	err := db.Insert(db.DefaultContext, grant)
	require.NoError(t, err)

	assert.Contains(t, grant.Scope, "openid read:user read:repository")

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=a&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]
	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "a",
		"code":          authcode,
	})
	accessTokenResp := ctx.MakeRequest(t, accessTokenReq, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	parsed := new(response)

	require.NoError(t, json.Unmarshal(accessTokenResp.Body.Bytes(), parsed))
	userReq := NewRequest(t, "GET", "/api/v1/users/user2/repos")
	userReq.SetHeader("Authorization", "Bearer "+parsed.AccessToken)
	userResp := MakeRequest(t, userReq, http.StatusOK)

	type repo struct {
		FullRepoName string `json:"full_name"`
		Private      bool   `json:"private"`
	}

	var reposCaptured []repo
	require.NoError(t, json.Unmarshal(userResp.Body.Bytes(), &reposCaptured))

	reposExpected := []repo{
		{
			FullRepoName: "user2/repo1",
			Private:      false,
		},
		{
			FullRepoName: "user2/repo2",
			Private:      true,
		},
		{
			FullRepoName: "user2/repo15",
			Private:      true,
		},
		{
			FullRepoName: "user2/repo16",
			Private:      true,
		},
		{
			FullRepoName: "user2/repo20",
			Private:      true,
		},
		{
			FullRepoName: "user2/utf8",
			Private:      false,
		},
		{
			FullRepoName: "user2/commits_search_test",
			Private:      false,
		},
		{
			FullRepoName: "user2/git_hooks_test",
			Private:      false,
		},
		{
			FullRepoName: "user2/glob",
			Private:      false,
		},
		{
			FullRepoName: "user2/lfs",
			Private:      true,
		},
		{
			FullRepoName: "user2/scoped_label",
			Private:      true,
		},
		{
			FullRepoName: "user2/readme-test",
			Private:      true,
		},
		{
			FullRepoName: "user2/repo-release",
			Private:      false,
		},
		{
			FullRepoName: "user2/commitsonpr",
			Private:      false,
		},
	}
	assert.Equal(t, reposExpected, reposCaptured)

	errorReq := NewRequest(t, "GET", "/api/v1/users/user2/orgs")
	errorReq.SetHeader("Authorization", "Bearer "+parsed.AccessToken)
	errorResp := MakeRequest(t, errorReq, http.StatusForbidden)

	type errorResponse struct {
		Message string `json:"message"`
	}

	errorParsed := new(errorResponse)
	require.NoError(t, json.Unmarshal(errorResp.Body.Bytes(), errorParsed))
	assert.Contains(t, errorParsed.Message, "token does not have at least one of required scope(s), required=[read:user read:organization]")
}

func TestOAuth_GrantScopesClaimPublicOnlyGroups(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"a",
		},
		ConfidentialClient: true,
	}

	appReq := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	appResp := MakeRequest(t, appReq, http.StatusCreated)

	var app *api.OAuth2Application
	DecodeJSON(t, appResp, &app)

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid groups read:user public-only",
	}

	err := db.Insert(db.DefaultContext, grant)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"openid", "groups", "read:user", "public-only"}, strings.Split(grant.Scope, " "))

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=a&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]

	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "a",
		"code":          authcode,
	})
	accessTokenResp := ctx.MakeRequest(t, accessTokenReq, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token,omitempty"`
	}
	parsed := new(response)
	require.NoError(t, json.Unmarshal(accessTokenResp.Body.Bytes(), parsed))
	parts := strings.Split(parsed.IDToken, ".")

	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	type IDTokenClaims struct {
		Groups []string `json:"groups"`
	}

	claims := new(IDTokenClaims)
	require.NoError(t, json.Unmarshal(payload, claims))

	userinfoReq := NewRequest(t, "GET", "/login/oauth/userinfo")
	userinfoReq.SetHeader("Authorization", "Bearer "+parsed.AccessToken)
	userinfoResp := MakeRequest(t, userinfoReq, http.StatusOK)

	type userinfoResponse struct {
		Login  string   `json:"login"`
		Email  string   `json:"email"`
		Groups []string `json:"groups"`
	}

	userinfoParsed := new(userinfoResponse)
	require.NoError(t, json.Unmarshal(userinfoResp.Body.Bytes(), userinfoParsed))
	assert.Contains(t, userinfoParsed.Email, "user2@example.com")

	// test both id_token and call to /login/oauth/userinfo
	for _, publicGroup := range []string{
		"org17",
		"org17:test_team",
		"org3",
		"org3:owners",
		"org3:team1",
		"org3:teamcreaterepo",
	} {
		assert.Contains(t, claims.Groups, publicGroup)
		assert.Contains(t, userinfoParsed.Groups, publicGroup)
	}
	for _, privateGroup := range []string{
		"private_org35",
		"private_org35_team24",
	} {
		assert.NotContains(t, claims.Groups, privateGroup)
		assert.NotContains(t, userinfoParsed.Groups, privateGroup)
	}
}

func TestOAuth_GrantScopesClaimAllGroups(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"a",
		},
		ConfidentialClient: true,
	}

	appReq := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	appResp := MakeRequest(t, appReq, http.StatusCreated)

	var app *api.OAuth2Application
	DecodeJSON(t, appResp, &app)

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid groups",
	}

	err := db.Insert(db.DefaultContext, grant)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"openid", "groups"}, strings.Split(grant.Scope, " "))

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=a&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]

	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "a",
		"code":          authcode,
	})
	accessTokenResp := ctx.MakeRequest(t, accessTokenReq, http.StatusOK)
	type response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token,omitempty"`
	}
	parsed := new(response)
	require.NoError(t, json.Unmarshal(accessTokenResp.Body.Bytes(), parsed))
	parts := strings.Split(parsed.IDToken, ".")

	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	type IDTokenClaims struct {
		Groups []string `json:"groups"`
	}

	claims := new(IDTokenClaims)
	require.NoError(t, json.Unmarshal(payload, claims))

	userinfoReq := NewRequest(t, "GET", "/login/oauth/userinfo")
	userinfoReq.SetHeader("Authorization", "Bearer "+parsed.AccessToken)
	userinfoResp := MakeRequest(t, userinfoReq, http.StatusOK)

	type userinfoResponse struct {
		Login  string   `json:"login"`
		Email  string   `json:"email"`
		Groups []string `json:"groups"`
	}

	userinfoParsed := new(userinfoResponse)
	require.NoError(t, json.Unmarshal(userinfoResp.Body.Bytes(), userinfoParsed))
	assert.Contains(t, userinfoParsed.Email, "user2@example.com")

	// test both id_token and call to /login/oauth/userinfo
	for _, group := range []string{
		"org17",
		"org17:test_team",
		"org3",
		"org3:owners",
		"org3:team1",
		"org3:teamcreaterepo",
		"private_org35",
		"private_org35:team24",
	} {
		assert.Contains(t, claims.Groups, group)
		assert.Contains(t, userinfoParsed.Groups, group)
	}
}
