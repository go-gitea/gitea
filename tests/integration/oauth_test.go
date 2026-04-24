// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/oauth2_provider"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testOAuth2PrepareTestCode(t *testing.T) {
	require.NoError(t, db.TruncateBeans(t.Context(), &auth_model.OAuth2AuthorizationCode{}))
	err := db.Insert(t.Context(), &auth_model.OAuth2AuthorizationCode{
		GrantID:             1,
		Code:                "authcode",
		CodeChallenge:       "CjvyTLSdR47G5zYenDA-eDWW4lRrO8yvjcWwbD_deOg", // Code Verifier: N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt
		CodeChallengeMethod: "S256",
		RedirectURI:         "https://example.com",
		ValidUntil:          timeutil.TimeStampNow() + 86400,
	}, &auth_model.OAuth2AuthorizationCode{
		GrantID:             4,
		Code:                "authcodepublic",
		CodeChallenge:       "CjvyTLSdR47G5zYenDA-eDWW4lRrO8yvjcWwbD_deOg", //# Code Verifier: N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt
		CodeChallengeMethod: "S256",
		RedirectURI:         "http://127.0.0.1/",
		ValidUntil:          timeutil.TimeStampNow() + 86400,
	})
	require.NoError(t, err)
}

func TestOAuth2(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Provider", func(t *testing.T) {
		t.Run("AuthorizeNoClientID", testAuthorizeNoClientID)
		t.Run("AuthorizeUnregisteredRedirect", testAuthorizeUnregisteredRedirect)
		t.Run("AuthorizeUnsupportedResponseType", testAuthorizeUnsupportedResponseType)
		t.Run("AuthorizeUnsupportedCodeChallengeMethod", testAuthorizeUnsupportedCodeChallengeMethod)
		t.Run("AuthorizeLoginRedirect", testAuthorizeLoginRedirect)
		t.Run("AuthorizeShow", testAuthorizeShow)
		t.Run("AuthorizeGrantS256RequiresVerifier", testAuthorizeGrantS256RequiresVerifier)
		t.Run("AuthorizeRedirectWithExistingGrant", testAuthorizeRedirectWithExistingGrant)
		t.Run("AuthorizePKCERequiredForPublicClient", testAuthorizePKCERequiredForPublicClient)
		t.Run("AccessTokenExchange", testAccessTokenExchange)
		t.Run("AccessTokenExchangeWithPublicClient", testAccessTokenExchangeWithPublicClient)
		t.Run("AccessTokenExchangeJSON", testAccessTokenExchangeJSON)
		t.Run("AccessTokenExchangeWithoutPKCE", testAccessTokenExchangeWithoutPKCE)
		t.Run("AccessTokenExchangeWithInvalidCredentials", testAccessTokenExchangeWithInvalidCredentials)
		t.Run("AccessTokenExchangeWithBasicAuth", testAccessTokenExchangeWithBasicAuth)
		t.Run("RefreshTokenInvalidation", testRefreshTokenInvalidation)
		t.Run("OAuthIntrospection", testOAuthIntrospection)
		t.Run("OAuthGrantScopesReadUserFailRepos", testOAuthGrantScopesReadUserFailRepos)
		t.Run("OAuthGrantScopesReadRepositoryFailOrganization", testOAuthGrantScopesReadRepositoryFailOrganization)
		t.Run("OAuthGrantScopesClaimPublicOnlyGroups", testOAuthGrantScopesClaimPublicOnlyGroups)
		t.Run("OAuthGrantScopesClaimAllGroups", testOAuthGrantScopesClaimAllGroups)
		t.Run("OAuth2WellKnown", testOAuth2WellKnown)
	})
	t.Run("Client", func(t *testing.T) {
		t.Run("OAuthSourceSpecialChars", testOAuthSourceSpecialChars)
		t.Run("SignInOauthCallbackSyncSSHKeys", testSignInOauthCallbackSyncSSHKeys)
	})
	// TODO: move more tests as sub-tests here, avoid unnecessary PrepareTestEnv
}

func testAuthorizeNoClientID(t *testing.T) {
	req := NewRequest(t, "GET", "/login/oauth/authorize")
	ctx := loginUser(t, "user2")
	resp := ctx.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Client ID not registered")
}

func testAuthorizeUnregisteredRedirect(t *testing.T) {
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=UNREGISTERED&response_type=code&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), "Unregistered Redirect URI")
}

func testAuthorizeUnsupportedResponseType(t *testing.T) {
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=https://example.com&response_type=UNEXPECTED&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "unsupported_response_type", u.Query().Get("error"))
	assert.Equal(t, "Only code response type is supported.", u.Query().Get("error_description"))
}

func testAuthorizeUnsupportedCodeChallengeMethod(t *testing.T) {
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=https://example.com&response_type=code&state=thestate&code_challenge_method=UNEXPECTED")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", u.Query().Get("error"))
	assert.Equal(t, "unsupported code challenge method", u.Query().Get("error_description"))
}

func testAuthorizeLoginRedirect(t *testing.T) {
	req := NewRequest(t, "GET", "/login/oauth/authorize")
	assert.Contains(t, MakeRequest(t, req, http.StatusSeeOther).Body.String(), "/user/login")
}

func testAuthorizeShow(t *testing.T) {
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=https://example.com&response_type=code&state=thestate")
	ctx := loginUser(t, "user4")
	resp := ctx.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	AssertHTMLElement(t, htmlDoc, "#authorize-app", true)
}

func testAuthorizeGrantS256RequiresVerifier(t *testing.T) {
	ctx := loginUser(t, "user4")
	codeChallenge := "CjvyTLSdR47G5zYenDA-eDWW4lRrO8yvjcWwbD_deOg"
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=https://example.com&response_type=code&state=thestate&code_challenge_method=S256&code_challenge="+url.QueryEscape(codeChallenge))
	resp := ctx.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	AssertHTMLElement(t, htmlDoc, "#authorize-app", true)

	grantReq := NewRequestWithValues(t, "POST", "/login/oauth/grant", map[string]string{
		"client_id":    "da7da3ba-9a13-4167-856f-3899de0b0138",
		"state":        "thestate",
		"scope":        "",
		"nonce":        "",
		"redirect_uri": "https://example.com",
		"granted":      "true",
	})
	grantResp := ctx.MakeRequest(t, grantReq, http.StatusSeeOther)
	u, err := grantResp.Result().Location()
	assert.NoError(t, err)
	code := u.Query().Get("code")
	assert.NotEmpty(t, code)

	accessReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "https://example.com",
		"code":          code,
	})
	accessResp := MakeRequest(t, accessReq, http.StatusBadRequest)
	parsedError := new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(accessResp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "failed PKCE code challenge", parsedError.ErrorDescription)
}

func testAuthorizeRedirectWithExistingGrant(t *testing.T) {
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=https://example.com/&response_type=code&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "thestate", u.Query().Get("state"))
	assert.Greaterf(t, len(u.Query().Get("code")), 30, "authorization code '%s' should be longer then 30", u.Query().Get("code"))
	u.RawQuery = ""
	assert.Equal(t, "https://example.com/", u.String())
}

func testAuthorizePKCERequiredForPublicClient(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	req := NewRequest(t, "GET", "/login/oauth/authorize?client_id=ce5a1322-42a7-11ed-b878-0242ac120002&redirect_uri=http%3A%2F%2F127.0.0.1&response_type=code&state=thestate")
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, http.StatusSeeOther)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", u.Query().Get("error"))
	assert.Equal(t, "PKCE is required for public clients", u.Query().Get("error_description"))
}

func testAccessTokenExchange(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "https://example.com",
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

func testAccessTokenExchangeWithPublicClient(t *testing.T) {
	testOAuth2PrepareTestCode(t)
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

func testAccessTokenExchangeJSON(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	req := NewRequestWithJSON(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "https://example.com",
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

func testAccessTokenExchangeWithoutPKCE(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "https://example.com",
		"code":          "authcode",
	})
	resp := MakeRequest(t, req, http.StatusBadRequest)
	parsedError := new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unauthorized_client", string(parsedError.ErrorCode))
	assert.Equal(t, "failed PKCE code challenge", parsedError.ErrorDescription)
}

func testAccessTokenExchangeWithInvalidCredentials(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	// invalid client id
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "???",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
		"code":          "authcode",
		"code_verifier": "N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt",
	})
	resp = MakeRequest(t, req, http.StatusBadRequest)
	parsedError = new(oauth2_provider.AccessTokenError)
	assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), parsedError))
	assert.Equal(t, "unsupported_grant_type", string(parsedError.ErrorCode))
	assert.Equal(t, "Only refresh_token or authorization_code grant type is supported", parsedError.ErrorDescription)
}

func testAccessTokenExchangeWithBasicAuth(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
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
		"redirect_uri": "https://example.com",
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
		"redirect_uri":  "https://example.com",
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

func testRefreshTokenInvalidation(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
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
		"redirect_uri":  "https://example.com",
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

func testOAuthIntrospection(t *testing.T) {
	testOAuth2PrepareTestCode(t)
	req := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "da7da3ba-9a13-4167-856f-3899de0b0138",
		"client_secret": "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=",
		"redirect_uri":  "https://example.com",
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

func testOAuthGrantScopesReadUserFailRepos(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"https://example.com",
		},
		ConfidentialClient: true,
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	app := DecodeJSON(t, resp, &api.OAuth2Application{})

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid read:user",
	}

	err := db.Insert(t.Context(), grant)
	require.NoError(t, err)

	assert.Contains(t, grant.Scope, "openid read:user")

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=https://example.com&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]

	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "https://example.com",
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

func testOAuthGrantScopesReadRepositoryFailOrganization(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"https://example.com",
		},
		ConfidentialClient: true,
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	app := DecodeJSON(t, resp, &api.OAuth2Application{})

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid read:user read:repository",
	}

	err := db.Insert(t.Context(), grant)
	require.NoError(t, err)

	assert.Contains(t, grant.Scope, "openid read:user read:repository")

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=https://example.com&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]
	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "https://example.com",
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

func testOAuthGrantScopesClaimPublicOnlyGroups(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"https://example.com",
		},
		ConfidentialClient: true,
	}

	appReq := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	appResp := MakeRequest(t, appReq, http.StatusCreated)

	app := DecodeJSON(t, appResp, &api.OAuth2Application{})

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid groups read:user public-only",
	}

	err := db.Insert(t.Context(), grant)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"openid", "groups", "read:user", "public-only"}, strings.Split(grant.Scope, " "))

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=https://example.com&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]

	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "https://example.com",
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

func testOAuthGrantScopesClaimAllGroups(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "oauth-provider-scopes-test",
		RedirectURIs: []string{
			"https://example.com",
		},
		ConfidentialClient: true,
	}

	appReq := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).
		AddBasicAuth(user.Name)
	appResp := MakeRequest(t, appReq, http.StatusCreated)

	app := DecodeJSON(t, appResp, &api.OAuth2Application{})

	grant := &auth_model.OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        user.ID,
		Scope:         "openid groups",
	}

	err := db.Insert(t.Context(), grant)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"openid", "groups"}, strings.Split(grant.Scope, " "))

	ctx := loginUser(t, user.Name)

	authorizeURL := fmt.Sprintf("/login/oauth/authorize?client_id=%s&redirect_uri=https://example.com&response_type=code&state=thestate", app.ClientID)
	authorizeReq := NewRequest(t, "GET", authorizeURL)
	authorizeResp := ctx.MakeRequest(t, authorizeReq, http.StatusSeeOther)

	authcode := strings.Split(strings.Split(authorizeResp.Body.String(), "?code=")[1], "&amp")[0]

	accessTokenReq := NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     app.ClientID,
		"client_secret": app.ClientSecret,
		"redirect_uri":  "https://example.com",
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

func testOAuth2WellKnown(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://try.gitea.io/")()
	urlOpenidConfiguration := "/.well-known/openid-configuration"

	t.Run("WellKnown", func(t *testing.T) {
		req := NewRequest(t, "GET", urlOpenidConfiguration)
		resp := MakeRequest(t, req, http.StatusOK)
		var respMap map[string]any
		DecodeJSON(t, resp, &respMap)
		assert.Equal(t, "https://try.gitea.io", respMap["issuer"])
		assert.Equal(t, "https://try.gitea.io/login/oauth/authorize", respMap["authorization_endpoint"])
		assert.Equal(t, "https://try.gitea.io/login/oauth/access_token", respMap["token_endpoint"])
		assert.Equal(t, "https://try.gitea.io/login/oauth/keys", respMap["jwks_uri"])
		assert.Equal(t, "https://try.gitea.io/login/oauth/userinfo", respMap["userinfo_endpoint"])
		assert.Equal(t, "https://try.gitea.io/login/oauth/introspect", respMap["introspection_endpoint"])
		assert.Equal(t, []any{"RS256"}, respMap["id_token_signing_alg_values_supported"])
	})

	t.Run("WellKnownWithIssuer", func(t *testing.T) {
		defer test.MockVariableValue(&setting.OAuth2.JWTClaimIssuer, "https://try.gitea.io/")()
		req := NewRequest(t, "GET", urlOpenidConfiguration)
		resp := MakeRequest(t, req, http.StatusOK)
		var respMap map[string]any
		DecodeJSON(t, resp, &respMap)
		assert.Equal(t, "https://try.gitea.io/", respMap["issuer"]) // has trailing by JWTClaimIssuer
		assert.Equal(t, "https://try.gitea.io/login/oauth/authorize", respMap["authorization_endpoint"])
	})

	defer test.MockVariableValue(&setting.OAuth2.Enabled, false)()
	MakeRequest(t, NewRequest(t, "GET", urlOpenidConfiguration), http.StatusNotFound)
}

func addOAuth2Source(t *testing.T, authName string, cfg oauth2.Source) {
	cfg.Provider = util.IfZero(cfg.Provider, "gitea")
	err := auth_model.CreateSource(t.Context(), &auth_model.Source{
		Type:     auth_model.OAuth2,
		Name:     authName,
		IsActive: true,
		Cfg:      &cfg,
	})
	require.NoError(t, err)
}

func createOAuth2MockProvider() *httptest.Server {
	var mockServer *httptest.Server
	mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = w.Write([]byte(`{
				"issuer": "` + mockServer.URL + `",
				"authorization_endpoint": "` + mockServer.URL + `/authorize",
				"token_endpoint": "` + mockServer.URL + `/token",
				"userinfo_endpoint": "` + mockServer.URL + `/userinfo"
			}`))
		default:
			http.NotFound(w, r)
		}
	}))

	return mockServer
}

func testSignInOauthCallbackSyncSSHKeys(t *testing.T) {
	mockServer := createOAuth2MockProvider()
	defer mockServer.Close()

	ctx := t.Context()
	oauth2Source := oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      "test-client-id",
		SSHPublicKeyClaimName:         "sshpubkey",
		FullNameClaimName:             "name",
		OpenIDConnectAutoDiscoveryURL: mockServer.URL + "/.well-known/openid-configuration",
	}
	addOAuth2Source(t, "test-oidc-source", oauth2Source)
	authSource, err := auth_model.GetActiveOAuth2SourceByAuthName(ctx, "test-oidc-source")
	require.NoError(t, err)

	sshKey1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf"
	sshKey2 := "sk-ssh-ed25519@openssh.com AAAAGnNrLXNzaC1lZDI1NTE5QG9wZW5zc2guY29tAAAAIE7kM1R02+4ertDKGKEDcKG0s+2vyDDcIvceJ0Gqv5f1AAAABHNzaDo="
	sshKey3 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEHjnNEfE88W1pvBLdV3otv28x760gdmPao3lVD5uAt9"
	cases := []struct {
		testName           string
		mockFullName       string
		mockRawData        map[string]any
		expectedSSHPubKeys []string
	}{
		{
			testName:           "Login1",
			mockFullName:       "FullName1",
			mockRawData:        map[string]any{"sshpubkey": []any{sshKey1 + " any-comment"}},
			expectedSSHPubKeys: []string{sshKey1},
		},
		{
			testName:           "Login2",
			mockFullName:       "FullName2",
			mockRawData:        map[string]any{"sshpubkey": []any{sshKey2 + " any-comment", sshKey3}},
			expectedSSHPubKeys: []string{sshKey2, sshKey3},
		},
		{
			testName:           "Login3",
			mockFullName:       "FullName3",
			mockRawData:        map[string]any{},
			expectedSSHPubKeys: []string{},
		},
	}

	session := emptyTestSession(t)
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			defer test.MockVariableValue(&setting.OAuth2Client.Username, "")()
			defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
			defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
				return goth.User{
					Provider: authSource.Cfg.(*oauth2.Source).Provider,
					UserID:   "oidc-userid",
					Email:    "oidc-email@example.com",
					RawData:  c.mockRawData,
					Name:     c.mockFullName,
				}, nil
			})()
			req := NewRequest(t, "GET", "/user/oauth2/test-oidc-source/callback?code=XYZ&state=XYZ")
			session.MakeRequest(t, req, http.StatusSeeOther)
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "oidc-userid"})
			keys, _, err := db.FindAndCount[asymkey_model.PublicKey](ctx, asymkey_model.FindPublicKeyOptions{
				ListOptions:   db.ListOptionsAll,
				OwnerID:       user.ID,
				LoginSourceID: authSource.ID,
			})
			require.NoError(t, err)
			var sshPubKeys []string
			for _, key := range keys {
				sshPubKeys = append(sshPubKeys, key.Content)
			}
			assert.ElementsMatch(t, c.expectedSSHPubKeys, sshPubKeys)
			assert.Equal(t, c.mockFullName, user.FullName)
		})
	}
}

// Checks if an OAuth provider with spaces within the name does work,
// with the encoding of its names in the URL (PR#37327)
func testOAuthSourceSpecialChars(t *testing.T) {
	mockServer := createOAuth2MockProvider()
	defer mockServer.Close()

	addOAuth2Source(t, "test space", oauth2.Source{
		Provider:                      "openidConnect",
		OpenIDConnectAutoDiscoveryURL: mockServer.URL + "/.well-known/openid-configuration",
	})
	addOAuth2Source(t, "test+plus", oauth2.Source{
		Provider:                      "openidConnect",
		OpenIDConnectAutoDiscoveryURL: mockServer.URL + "/.well-known/openid-configuration",
	})

	testOAuth2 := func(t *testing.T, uri string, statusCode int) {
		req := NewRequest(t, "GET", uri)
		resp := MakeRequest(t, req, statusCode)
		if statusCode == http.StatusTemporaryRedirect {
			assert.NotEmpty(t, resp.Header().Get("Location"))
		} else {
			assert.Empty(t, resp.Header().Get("Location"))
		}
	}

	req := MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusOK)
	doc := NewHTMLParser(t, req.Body)
	var oauth2Links []string
	doc.Find(".external-login-link").Each(func(i int, s *goquery.Selection) {
		oauth2Links = append(oauth2Links, s.AttrOr("href", ""))
	})
	assert.Equal(t, []string{
		"/user/oauth2/test%20space",
		"/user/oauth2/test+plus",
	}, oauth2Links)

	testOAuth2(t, "/user/oauth2/test%20space", http.StatusTemporaryRedirect)
	testOAuth2(t, "/user/oauth2/test+space", http.StatusNotFound)

	testOAuth2(t, "/user/oauth2/test+plus", http.StatusTemporaryRedirect)
	testOAuth2(t, "/user/oauth2/test%2Bplus", http.StatusTemporaryRedirect)
	testOAuth2(t, "/user/oauth2/test%20plus", http.StatusNotFound)
}
