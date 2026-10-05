// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/routers"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getSessionID(resp *httptest.ResponseRecorder) string {
	for _, cookie := range resp.Result().Cookies() {
		if cookie.Name == setting.SessionConfig.CookieName {
			return cookie.Value
		}
	}
	return ""
}

func TestSessionFileCreation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	tmpDir := t.TempDir()
	defer test.MockVariableValue(&setting.SessionConfig.Provider, "file")()
	defer test.MockVariableValue(&setting.SessionConfig.ProviderConfig, tmpDir)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	t.Run("NoSessionOnViewIssue", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1"), http.StatusOK)
		assert.Empty(t, getSessionID(resp))
	})
	t.Run("CreateSessionOnLogin", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		resp := MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusOK)
		assert.Empty(t, getSessionID(resp))

		req := NewRequestWithValues(t, "POST", "/user/login", map[string]string{
			"user_name": "user2",
			"password":  userPassword,
		})
		sessionID := getSessionID(MakeRequest(t, req, http.StatusSeeOther))
		require.Len(t, sessionID, 16)
		assert.FileExists(t, filepath.Join(tmpDir, sessionID[0:1], sessionID[1:2], sessionID))
	})
}
