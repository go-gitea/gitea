// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

type ldapUser struct {
	UserName    string
	Password    string
	FullName    string
	Email       string
	OtherEmails []string
	IsAdmin     bool
	SSHKeys     []string
}

var gitLDAPUsers = []ldapUser{
	{
		UserName:    "professor",
		Password:    "professor",
		FullName:    "Hubert Farnsworth",
		Email:       "professor@planetexpress.com",
		OtherEmails: []string{"hubert@planetexpress.com"},
		IsAdmin:     true,
	},
	{
		UserName: "hermes",
		Password: "hermes",
		FullName: "Conrad Hermes",
		Email:    "hermes@planetexpress.com",
		SSHKeys: []string{
			"SHA256:qLY06smKfHoW/92yXySpnxFR10QFrLdRjf/GNPvwcW8",
			"SHA256:QlVTuM5OssDatqidn2ffY+Lc4YA5Fs78U+0KOHI51jQ",
			"SHA256:DXdeUKYOJCSSmClZuwrb60hUq7367j4fA+udNC3FdRI",
		},
		IsAdmin: true,
	},
	{
		UserName: "fry",
		Password: "fry",
		FullName: "Philip Fry",
		Email:    "fry@planetexpress.com",
	},
	{
		UserName: "leela",
		Password: "leela",
		FullName: "Leela Turanga",
		Email:    "leela@planetexpress.com",
	},
	{
		UserName: "bender",
		Password: "bender",
		FullName: "Bender Rodríguez",
		Email:    "bender@planetexpress.com",
	},
}

var otherLDAPUsers = []ldapUser{
	{
		UserName: "zoidberg",
		Password: "zoidberg",
		FullName: "John Zoidberg",
		Email:    "zoidberg@planetexpress.com",
	},
	{
		UserName: "amy",
		Password: "amy",
		FullName: "Amy Kroker",
		Email:    "amy@planetexpress.com",
	},
}

func skipLDAPTests() bool {
	return os.Getenv("TEST_LDAP") != "1"
}

func getLDAPServerHost() string {
	host := os.Getenv("TEST_LDAP_HOST")
	if len(host) == 0 {
		host = "ldap"
	}
	return host
}

func addAuthSourceLDAP(t *testing.T, sshKeyAttribute string) {
	session := loginUser(t, "user1")
	csrf := GetCSRF(t, session, "/admin/auths/new")
	req := NewRequestWithValues(t, "POST", "/admin/auths/new", map[string]string{
		"_csrf":                    csrf,
		"type":                     "2",
		"name":                     "ldap",
		"host":                     getLDAPServerHost(),
		"port":                     "389",
		"bind_dn":                  "uid=gitea,ou=service,dc=planetexpress,dc=com",
		"bind_password":            "password",
		"user_base":                "ou=people,dc=planetexpress,dc=com",
		"filter":                   "(&(objectClass=inetOrgPerson)(memberOf=cn=git,ou=people,dc=planetexpress,dc=com)(uid=%s))",
		"admin_filter":             "(memberOf=cn=admin_staff,ou=people,dc=planetexpress,dc=com)",
		"attribute_username":       "uid",
		"attribute_name":           "givenName",
		"attribute_surname":        "sn",
		"attribute_mail":           "mail",
		"attribute_ssh_public_key": sshKeyAttribute,
		"is_sync_enabled":          "on",
		"is_active":                "on",
	})
	session.MakeRequest(t, req, http.StatusFound)
}

func TestLDAPUserSignin(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")

	u := gitLDAPUsers[0]

	session := loginUserWithPassword(t, u.UserName, u.Password)
	req := NewRequest(t, "GET", "/user/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	assert.Equal(t, u.UserName, htmlDoc.GetInputValueByName("name"))
	assert.Equal(t, u.FullName, htmlDoc.GetInputValueByName("full_name"))
	assert.Equal(t, u.Email, htmlDoc.GetInputValueByName("email"))
}

func TestLDAPUserSync(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")
	models.SyncExternalUsers(context.Background())

	session := loginUser(t, "user1")
	// Check if users exists
	for _, u := range gitLDAPUsers {
		req := NewRequest(t, "GET", "/admin/users?q="+u.UserName)
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)

		tr := htmlDoc.doc.Find("table.table tbody tr")
		if !assert.True(t, tr.Length() == 1) {
			continue
		}
		tds := tr.Find("td")
		if !assert.True(t, tds.Length() > 0) {
			continue
		}
		assert.Equal(t, u.UserName, strings.TrimSpace(tds.Find("td:nth-child(2) a").Text()))
		assert.Equal(t, u.Email, strings.TrimSpace(tds.Find("td:nth-child(3) span").Text()))
		if u.IsAdmin {
			assert.True(t, tds.Find("td:nth-child(5) i").HasClass("fa-check-square-o"))
		} else {
			assert.True(t, tds.Find("td:nth-child(5) i").HasClass("fa-square-o"))
		}
	}

	// Check if no users exist
	for _, u := range otherLDAPUsers {
		req := NewRequest(t, "GET", "/admin/users?q="+u.UserName)
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)

		tr := htmlDoc.doc.Find("table.table tbody tr")
		assert.True(t, tr.Length() == 0)
	}
}

func TestLDAPUserSigninFailed(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")

	u := otherLDAPUsers[0]

	testLoginFailed(t, u.UserName, u.Password, i18n.Tr("en", "form.username_password_incorrect"))
}

func TestLDAPUserSSHKeySync(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "sshPublicKey")

	models.SyncExternalUsers(context.Background())

	// Check if users has SSH keys synced
	for _, u := range gitLDAPUsers {
		if len(u.SSHKeys) == 0 {
			continue
		}
		session := loginUserWithPassword(t, u.UserName, u.Password)

		req := NewRequest(t, "GET", "/user/settings/keys")
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)

		divs := htmlDoc.doc.Find(".key.list .print.meta")

		syncedKeys := make([]string, divs.Length())
		for i := 0; i < divs.Length(); i++ {
			syncedKeys[i] = strings.TrimSpace(divs.Eq(i).Text())
		}

		assert.ElementsMatch(t, u.SSHKeys, syncedKeys)
	}
}
