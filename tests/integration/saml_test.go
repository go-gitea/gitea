// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSAMLRegistration(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// check the saml metadata url
	req := NewRequest(t, "GET", "/user/saml/test-sp/metadata")
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user/saml/test-sp")
	resp := MakeRequest(t, req, http.StatusTemporaryRedirect)

	jar, err := cookiejar.New(nil)
	assert.NoError(t, err)

	client := http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}

	req, err = http.NewRequest("GET", test.RedirectURL(resp), nil)
	assert.NoError(t, err)

	res, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	// find the auth state hidden input
	authStateMatcher := regexp.MustCompile(`<input.*?name="AuthState".*?value="([^"]+)".*?>`)
	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)

	matches := authStateMatcher.FindStringSubmatch(string(body))
	assert.Len(t, matches, 2)
	assert.NoError(t, res.Body.Close())

	form := url.Values{
		"username":  {"user1"},
		"password":  {"user1pass"},
		"AuthState": {html.UnescapeString(matches[1])},
	}

	req, err = http.NewRequest("POST", "http://localhost:8080/simplesaml/module.php/core/loginuserpass.php", strings.NewReader(form.Encode()))
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	samlResMatcher := regexp.MustCompile(`<input.*?value="([^"]+)".*?>`)

	body, err = io.ReadAll(res.Body)
	assert.NoError(t, err)

	matches = samlResMatcher.FindStringSubmatch(string(body))
	assert.Len(t, matches, 2)
	assert.NoError(t, res.Body.Close())

	session := emptyTestSession(t)

	req = NewRequestWithValues(t, "POST", "/user/saml/test-sp/acs", map[string]string{
		"SAMLResponse": matches[1],
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, test.RedirectURL(resp), "/user/link_account")

	csrf := GetCSRF(t, session, test.RedirectURL(resp))

	// link the account
	req = NewRequestWithValues(t, "POST", "/user/link_account_signup", map[string]string{
		"_csrf":     csrf,
		"user_name": "samluser",
		"email":     "saml@example.com",
	})

	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, test.RedirectURL(resp), "/")

	// verify that the user was created
	u, err := user_model.GetUserByEmail(db.DefaultContext, "saml@example.com")
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, "samluser", u.Name)
}
