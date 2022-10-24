// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// token has to be unique this counter take care of
var tokenCounter int64

func getTokenForLoggedInUser(t testing.TB, session *TestSession) string {
	t.Helper()
	req := NewRequest(t, "GET", "/user/settings/applications")
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/settings/applications", map[string]string{
		"_csrf": doc.GetCSRF(),
		"name":  fmt.Sprintf("api-testing-token-%d", atomic.AddInt64(&tokenCounter, 1)),
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", "/user/settings/applications")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	token := htmlDoc.doc.Find(".ui.info p").Text()
	assert.NotEmpty(t, token)
	return token
}

func getUserToken(t testing.TB, userName string) string {
	return getTokenForLoggedInUser(t, loginUser(t, userName))
}

type APITestContext struct {
	Reponame     string
	Token        string
	Username     string
	ExpectedCode int
}

func NewAPITestContext(t testing.TB, username, reponame string) *APITestContext {
	session := loginUser(t, username)
	token := getTokenForLoggedInUser(t, session)
	return &APITestContext{
		Token:    token,
		Username: username,
		Reponame: reponame,
	}
}

func (ctx *APITestContext) CreateTestContext(t testing.TB) *TestContext {
	return NewTestContext(t, ctx.Username, ctx.Reponame)
}

func (ctx *APITestContext) GitPath() string {
	return fmt.Sprintf("%s/%s.git", ctx.Username, ctx.Reponame)
}

func (ctx *APITestContext) MakeRequest(t testing.TB, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	token := req.URL.Query().Get("token")
	if token == "" {
		values := req.URL.Query()
		values.Set("token", ctx.Token)
		req.URL.RawQuery = values.Encode()
	}
	return MakeRequest(t, req, expectedStatus)
}
