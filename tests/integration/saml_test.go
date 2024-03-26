// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
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
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/auth/source/saml"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSAMLRegistration(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	samlURL := "localhost:8080"

	if os.Getenv("CI") == "" || !setting.Database.Type.IsPostgreSQL() {
		// Make it possible to run tests against a local simplesaml instance
		samlURL = os.Getenv("TEST_SIMPLESAML_URL")
		if samlURL == "" {
			t.Skip("TEST_SIMPLESAML_URL not set and not running in CI")
			return
		}
	}

	privateKey, cert, err := saml.GenerateSAMLSPKeypair()
	assert.NoError(t, err)

	// verify that the keypair can be parsed
	keyPair, err := tls.X509KeyPair([]byte(cert), []byte(privateKey))
	assert.NoError(t, err)
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	assert.NoError(t, err)

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
			ServiceProviderCertificate:               "", // SimpleSAMLPhp requires that the SP certificate be specified in the server configuration rather than SP metadata
			ServiceProviderPrivateKey:                "",
			EmailAssertionKey:                        "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
			NameAssertionKey:                         "http://schemas.xmlsoap.org/claims/CommonName",
			UsernameAssertionKey:                     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
			IconURL:                                  "",
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

	httpReq, err := http.NewRequest("GET", test.RedirectURL(resp), nil)
	assert.NoError(t, err)

	var formRedirectURL *url.URL
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// capture the redirected destination to use in POST request
		formRedirectURL = req.URL
		return nil
	}

	res, err := client.Do(httpReq)
	client.CheckRedirect = nil
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.NotNil(t, formRedirectURL)

	form := url.Values{
		"username": {"user1"},
		"password": {"user1pass"},
	}

	httpReq, err = http.NewRequest("POST", formRedirectURL.String(), strings.NewReader(form.Encode()))
	assert.NoError(t, err)
	httpReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(httpReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)

	samlResMatcher := regexp.MustCompile(`<input.*?name="SAMLResponse".*?value="([^"]+)".*?>`)
	matches := samlResMatcher.FindStringSubmatch(string(body))
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
