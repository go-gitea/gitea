// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const userPassword = "password"

var (
	loginSessionCache = make(map[string]*TestSession, 10)
	loginSessionLock  sync.RWMutex
)

func emptyTestSession(t testing.TB) *TestSession {
	t.Helper()
	jar, err := cookiejar.New(nil)
	assert.NoError(t, err)

	return &TestSession{jar: jar}
}

func loginUser(t testing.TB, userName string) *TestSession {
	t.Helper()
	loginSessionLock.RLock()
	if session, ok := loginSessionCache[userName]; ok {
		loginSessionLock.RUnlock()
		return session
	}
	session := loginUserWithPassword(t, userName, userPassword)
	loginSessionLock.Lock()
	loginSessionCache[userName] = session
	loginSessionLock.Unlock()
	return session
}

func loginUserWithPassword(t testing.TB, userName, password string) *TestSession {
	t.Helper()
	req := NewRequest(t, "GET", "/user/login")
	resp := MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"_csrf":     doc.GetCSRF(),
		"user_name": userName,
		"password":  password,
	})
	resp = MakeRequest(t, req, http.StatusSeeOther)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session := emptyTestSession(t)

	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	return session
}

type TestSession struct {
	jar http.CookieJar
}

func (s *TestSession) GetCookie(name string) *http.Cookie {
	baseURL, err := url.Parse(setting.AppURL)
	if err != nil {
		return nil
	}

	for _, c := range s.jar.Cookies(baseURL) {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (s *TestSession) MakeRequest(t testing.TB, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	t.Helper()
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	for _, c := range s.jar.Cookies(baseURL) {
		req.AddCookie(c)
	}
	resp := MakeRequest(t, req, expectedStatus)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}
	s.jar.SetCookies(baseURL, cr.Cookies())

	return resp
}

func (s *TestSession) MakeRequestNilResponseRecorder(t testing.TB, req *http.Request, expectedStatus int) *NilResponseRecorder {
	t.Helper()
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	for _, c := range s.jar.Cookies(baseURL) {
		req.AddCookie(c)
	}
	resp := MakeRequestNilResponseRecorder(t, req, expectedStatus)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}
	s.jar.SetCookies(baseURL, cr.Cookies())

	return resp
}

func (s *TestSession) MakeRequestNilResponseHashSumRecorder(t testing.TB, req *http.Request, expectedStatus int) *NilResponseHashSumRecorder {
	t.Helper()
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	for _, c := range s.jar.Cookies(baseURL) {
		req.AddCookie(c)
	}
	resp := MakeRequestNilResponseHashSumRecorder(t, req, expectedStatus)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}
	s.jar.SetCookies(baseURL, cr.Cookies())

	return resp
}

type TestContext struct {
	Username string
	Reponame string
	Session  *TestSession
}

func NewTestContext(t testing.TB, username, reponame string) *TestContext {
	session := loginUser(t, username)
	return &TestContext{
		Session:  session,
		Username: username,
		Reponame: reponame,
	}
}

func (ctx *TestContext) GitPath() string {
	return fmt.Sprintf("%s/%s.git", ctx.Username, ctx.Reponame)
}

func (ctx *TestContext) CreateAPITestContext(t testing.TB) *APITestContext {
	return NewAPITestContext(t, ctx.Username, ctx.Reponame)
}

func (ctx *TestContext) MakeRequest(t testing.TB, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	return ctx.Session.MakeRequest(t, req, expectedStatus)
}

func GetCSRF(t testing.TB, session *TestSession, urlStr string) string {
	t.Helper()
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	return doc.GetCSRF()
}
