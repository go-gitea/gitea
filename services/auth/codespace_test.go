// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	codespace_service "gitea.dev/services/codespace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodespaceTokenBasicAuth(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, codespaceUUID := createAuthCodespaceToken(t)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v1/user", nil)
	require.NoError(t, err)
	req.SetBasicAuth(token, "x-oauth-basic")
	store := make(reqctx.ContextData)

	u, err := new(Basic).Verify(req, nil, store, nil)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.EqualValues(t, 1, u.ID)
	assert.Equal(t, CodespaceTokenMethodName, store.GetData()["LoginMethod"])
	assert.Equal(t, true, store.GetData()["IsApiToken"])
	scope := store.GetData()["ApiTokenScope"].(auth_model.AccessTokenScope)
	assertContainsCodespaceScopes(t, scope)
	snapshot := store.GetData()[codespace_model.GiteaTokenAuthDataKey].(*codespace_service.GiteaTokenAuthSnapshot)
	assert.Equal(t, codespaceUUID, snapshot.CodespaceUUID)
	assert.EqualValues(t, 2, snapshot.RepoID)
}

func TestCodespaceTokenBearerAuth(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v1/user", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	store := make(reqctx.ContextData)

	u, err := new(OAuth2).Verify(req, nil, store, nil)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.EqualValues(t, 1, u.ID)
	assert.Equal(t, CodespaceTokenMethodName, store.GetData()["LoginMethod"])
	assertContainsCodespaceScopes(t, store.GetData()["ApiTokenScope"].(auth_model.AccessTokenScope))
}

func TestCodespaceTokenQueryAuthIsIgnored(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	for _, queryName := range []string{"token", "access_token"} {
		req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v1/user?"+queryName+"="+token, nil)
		require.NoError(t, err)

		u, err := new(OAuth2).Verify(req, nil, make(reqctx.ContextData), nil)
		require.NoError(t, err)
		assert.Nil(t, u)
	}
}

func TestCodespaceTokenBasicAuthHonorsWebRoutePermission(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/user2/repo1/releases/download/v1/file.zip", nil)
	require.NoError(t, err)
	req = req.WithContext(reqctx.NewRequestContextForTest(req.Context()))
	req.SetBasicAuth(token, "x-oauth-basic")
	SetCodespaceTokenAuthAllowed(req.Context(), false)

	u, err := new(Basic).Verify(req, nil, make(reqctx.ContextData), nil)
	assert.Nil(t, u)
	require.Error(t, err)
	assert.True(t, IsCodespaceTokenForbidden(err))
}

func TestCodespaceTokenBasicAuthAllowsMarkedWebRoute(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/user2/repo1.git/info/refs", nil)
	require.NoError(t, err)
	req = req.WithContext(reqctx.NewRequestContextForTest(req.Context()))
	req.SetBasicAuth(token, "x-oauth-basic")
	SetCodespaceTokenAuthAllowed(req.Context(), true)

	u, err := new(Basic).Verify(req, nil, make(reqctx.ContextData), nil)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.EqualValues(t, 1, u.ID)
}

func TestCodespaceTokenBearerAuthAllowsMarkedWebRoute(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/user2/repo1.git/info/lfs/objects/batch", nil)
	require.NoError(t, err)
	req = req.WithContext(reqctx.NewRequestContextForTest(req.Context()))
	req.Header.Set("Authorization", "Bearer "+token)
	SetCodespaceTokenAuthAllowed(req.Context(), true)
	group := NewGroup(&Basic{}, &CodespaceToken{})

	u, err := group.Verify(req, nil, make(reqctx.ContextData), nil)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.EqualValues(t, 1, u.ID)
}

func TestCodespaceTokenQueryAuthHonorsDisableQueryToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	original := setting.DisableQueryAuthToken
	setting.DisableQueryAuthToken = true
	t.Cleanup(func() { setting.DisableQueryAuthToken = original })
	req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v1/user?token="+token, nil)
	require.NoError(t, err)

	u, err := new(OAuth2).Verify(req, nil, make(reqctx.ContextData), nil)
	require.NoError(t, err)
	assert.Nil(t, u)
}

func TestCodespaceTokenQueryAuthIgnoredForWebAuth(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/user2/repo1?token="+token, nil)
	require.NoError(t, err)
	req = req.WithContext(reqctx.NewRequestContextForTest(req.Context()))
	SetCodespaceTokenAuthAllowed(req.Context(), true)

	u, err := new(OAuth2).Verify(req, nil, make(reqctx.ContextData), nil)
	require.NoError(t, err)
	assert.Nil(t, u)

	u, err = new(CodespaceToken).Verify(req, nil, make(reqctx.ContextData), nil)
	require.NoError(t, err)
	assert.Nil(t, u)
}

func TestCodespaceTokenRejectedStopsAuthGroupFallback(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v1/user", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer gcs_bad")
	fallback := &fallbackAuthMethod{}
	group := NewGroup(&OAuth2{}, fallback)

	u, err := group.Verify(req, nil, make(reqctx.ContextData), nil)
	assert.Nil(t, u)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthMethodTerminal)
	assert.False(t, fallback.called)
}

func TestCodespaceTokenRejectValidStopsFallback(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, _ := createAuthCodespaceToken(t)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/user/settings", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	fallback := &fallbackAuthMethod{}
	group := NewGroup(&CodespaceToken{RejectValid: true}, fallback)

	u, err := group.Verify(req, nil, make(reqctx.ContextData), nil)
	assert.Nil(t, u)
	require.Error(t, err)
	assert.True(t, IsCodespaceTokenForbidden(err))
	assert.False(t, fallback.called)
}

func TestCodespaceTokenUnavailableStateIsForbidden(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	token, codespaceUUID := createAuthCodespaceToken(t)
	_, err := db.GetEngine(t.Context()).
		ID(codespaceUUID).
		Cols("status").
		Update(&codespace_model.Codespace{Status: codespace_model.StatusStopped})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodGet, "https://example.test/api/v1/user", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	u, err := new(OAuth2).Verify(req, nil, make(reqctx.ContextData), nil)
	assert.Nil(t, u)
	require.Error(t, err)
	assert.True(t, IsCodespaceTokenForbidden(err))
}

type fallbackAuthMethod struct {
	called bool
}

func (m *fallbackAuthMethod) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	m.called = true
	return user_model.NewGhostUser(), nil
}

func (m *fallbackAuthMethod) Name() string {
	return "fallback"
}

func createAuthCodespaceToken(t *testing.T) (string, string) {
	t.Helper()
	original := setting.Codespace.Enabled
	setting.Codespace.Enabled = true
	t.Cleanup(func() { setting.Codespace.Enabled = original })

	manager := &codespace_model.Manager{
		Name:           "manager",
		RuntimeState:   codespace_model.ManagerRuntimeStateOnline,
		TagsJSON:       "[]",
		CreatedUnix:    time.Now().Unix(),
		LastOnlineUnix: time.Now().Unix(),
		MetaJSON:       "{}",
	}
	manager.GenerateManagerSecret()
	require.NoError(t, db.Insert(t.Context(), manager))

	codespaceUUID := "12121212-1212-4212-8212-121212121212"
	require.NoError(t, db.Insert(t.Context(), &codespace_model.Codespace{
		UUID:              codespaceUUID,
		UserID:            1,
		RepoID:            2,
		RefType:           "branch",
		RefName:           "main",
		RepoTag:           "default",
		GitProtocol:       codespace_model.GitProtocolHTTP,
		CommitSHA:         "0123456789abcdef0123456789abcdef01234567",
		ManagerID:         manager.ID,
		Status:            codespace_model.StatusRunning,
		OperationRVersion: 1,
		AutoStopMode:      codespace_model.AutoStopModeDefault,
		CreatedUnix:       time.Now().Unix(),
		UpdatedUnix:       time.Now().Unix(),
		LogFilename:       codespaceUUID + ".log",
	}))
	result, err := codespace_service.RequestGiteaToken(t.Context(), manager, codespace_service.RequestGiteaTokenOptions{
		CodespaceUUID: codespaceUUID,
	})
	require.NoError(t, err)
	return result.Token, codespaceUUID
}

func assertContainsCodespaceScopes(t *testing.T, scope auth_model.AccessTokenScope) {
	t.Helper()
	for _, required := range []auth_model.AccessTokenScope{
		auth_model.AccessTokenScopeWriteIssue,
		auth_model.AccessTokenScopeWriteRepository,
		auth_model.AccessTokenScopeReadUser,
	} {
		ok, err := scope.HasScope(required)
		require.NoError(t, err)
		assert.True(t, ok)
	}
	ok, err := scope.HasScope(auth_model.AccessTokenScopeWriteUser)
	require.NoError(t, err)
	assert.False(t, ok)
}
