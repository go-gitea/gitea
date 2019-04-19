// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/macaron.v1"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/routes"

	"github.com/go-macaron/session"
	"github.com/stretchr/testify/assert"
)

func makeRequest(t testing.TB, localMac *macaron.Macaron, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	localMac.ServeHTTP(recorder, req)
	if expectedStatus != NoExpectedStatus {
		if !assert.EqualValues(t, expectedStatus, recorder.Code,
			"Request: %s %s", req.Method, req.URL.String()) {
			logUnexpectedResponse(t, recorder)
		}
	}
	return recorder
}

func getSessionID(t *testing.T, resp *httptest.ResponseRecorder) string {
	cookies := resp.Result().Cookies()
	found := false
	sessionID := ""
	for _, cookie := range cookies {
		if cookie.Name == setting.SessionConfig.CookieName {
			sessionID = cookie.Value
			found = true
		}
	}
	assert.True(t, found)
	assert.NotEmpty(t, sessionID)
	return sessionID
}

func sessionFile(tmpDir, sessionID string) string {
	return filepath.Join(tmpDir, sessionID[0:1], sessionID[1:2], sessionID)
}

func sessionFileExist(t *testing.T, tmpDir, sessionID string) bool {
	sessionFile := sessionFile(tmpDir, sessionID)
	_, err := os.Lstat(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		assert.NoError(t, err)
	}
	return true
}

func TestSessionFileCreation(t *testing.T) {
	prepareTestEnv(t)

	oldSessionConfig := setting.SessionConfig.ProviderConfig
	defer func() {
		setting.SessionConfig.ProviderConfig = oldSessionConfig
	}()

	var config session.Options
	err := json.Unmarshal([]byte(oldSessionConfig), &config)
	assert.NoError(t, err)

	config.Provider = "file"

	// Now create a temporaryDirectory
	tmpDir, err := ioutil.TempDir("", "sessions")
	assert.NoError(t, err)
	defer func() {
		if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
			_ = os.RemoveAll(tmpDir)
		}
	}()
	config.ProviderConfig = tmpDir

	newConfigBytes, err := json.Marshal(config)
	assert.NoError(t, err)

	setting.SessionConfig.ProviderConfig = string(newConfigBytes)

	localMac := routes.NewMacaron()
	routes.RegisterRoutes(localMac)

	t.Run("NoSessionOnViewIssue", func(t *testing.T) {
		PrintCurrentTest(t)

		req := NewRequest(t, "GET", "/user2/repo1/issues/1")
		resp := makeRequest(t, localMac, req, http.StatusOK)
		sessionID := getSessionID(t, resp)

		// We're not logged in so there should be no session
		assert.False(t, sessionFileExist(t, tmpDir, sessionID))
	})
	t.Run("CreateSessionOnLogin", func(t *testing.T) {
		PrintCurrentTest(t)

		req := NewRequest(t, "GET", "/user/login")
		resp := makeRequest(t, localMac, req, http.StatusOK)
		sessionID := getSessionID(t, resp)

		// We're not logged in so there should be no session
		assert.False(t, sessionFileExist(t, tmpDir, sessionID))

		doc := NewHTMLParser(t, resp.Body)
		req = NewRequestWithValues(t, "POST", "/user/login", map[string]string{
			"_csrf":     doc.GetCSRF(),
			"user_name": "user2",
			"password":  userPassword,
		})
		resp = makeRequest(t, localMac, req, http.StatusFound)
		sessionID = getSessionID(t, resp)

		assert.FileExists(t, sessionFile(tmpDir, sessionID))
	})
}
