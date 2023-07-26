// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/web/auth"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
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
	htmlDoc.AssertElement(t, "#authorize-app", true)
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
	assert.Truef(t, len(u.Query().Get("code")) > 30, "authorization code '%s' should be longer then 30", u.Query().Get("code"))
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
	assert.True(t, len(parsed.AccessToken) > 10)
	assert.True(t, len(parsed.RefreshToken) > 10)
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
	assert.True(t, len(parsed.AccessToken) > 10)
	assert.True(t, len(parsed.RefreshToken) > 10)
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
	assert.True(t, len(parsed.AccessToken) > 10)
	assert.True(t, len(parsed.RefreshToken) > 10)
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
	parsedError := new(auth.AccessTokenError)
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
	parsedError := new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	assert.True(t, len(parsed.AccessToken) > 10)
	assert.True(t, len(parsed.RefreshToken) > 10)

	// use wrong client_secret
	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"redirect_uri":  "a",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	req.Header.Add("Authorization", "Basic ZGE3ZGEzYmEtOWExMy00MTY3LTg1NmYtMzg5OWRlMGIwMTM4OmJsYWJsYQ==")
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError := new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	parsedError := new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
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
	parsedError = new(auth.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "token was already used", parsedError.ErrorDescription)
}
