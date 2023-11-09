// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/auth/source/saml"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSAMLRegistration(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	samlURL := "127.0.0.1:8080"

	if os.Getenv("CI") == "" {
		// Make it possible to run tests against a local simplesaml instance
		samlURL = os.Getenv("TEST_SIMPLESAML_URL")
		if samlURL == "" {
			t.Skip("TEST_SIMPLESAML_URL not set and not running in CI")
			return
		}
	}

	assert.NoError(t, auth.CreateSource(db.DefaultContext, &auth.Source{
		Type:          auth.SAML,
		Name:          "test-sp",
		IsActive:      true,
		IsSyncEnabled: false,
		Cfg: &saml.Source{
			IdentityProviderMetadata:                 "",
			IdentityProviderMetadataURL:              fmt.Sprintf("http://%s/simplesaml/saml2/idp/metadata.php", samlURL),
			InsecureSkipAssertionSignatureValidation: false,
			NameIDFormat:                             4,
			ServiceProviderCertificate:               "",
			ServiceProviderPrivateKey:                "",
			SignRequests:                             false,
			EmailAssertionKey:                        "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
			NameAssertionKey:                         "http://schemas.xmlsoap.org/claims/CommonName",
			UsernameAssertionKey:                     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		},
	}))

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

	req, err = http.NewRequest("POST", fmt.Sprintf("http://%s/simplesaml/module.php/core/loginuserpass.php", samlURL), strings.NewReader(form.Encode()))
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
