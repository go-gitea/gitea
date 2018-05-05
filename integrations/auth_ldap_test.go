// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"os"
	"testing"
)

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

func addAuthSourceLDAP(t *testing.T) {
	session := loginUser(t, "user1")
	csrf := GetCSRF(t, session, "/admin/auths/new")
	req := NewRequestWithValues(t, "POST", "/admin/auths/new", map[string]string{
		"_csrf":              csrf,
		"type":               "2",
		"name":               "ldap",
		"host":               getLDAPServerHost(),
		"port":               "389",
		"bind_dn":            "uid=gitea,ou=service,dc=planetexpress,dc=com",
		"bind_password":      "password",
		"user_base":          "ou=people,dc=planetexpress,dc=com",
		"filter":             "(&(objectClass=inetOrgPerson)(memberOf=cn=git,ou=people,dc=planetexpress,dc=com)(uid=%s))",
		"admin_filter":       "(memberOf=cn=admin_staff,ou=people,dc=planetexpress,dc=com)",
		"attribute_username": "uid",
		"attribute_name":     "cn",
		"attribute_surname":  "sn",
		"attribute_mail":     "mail",
		"is_sync_enabled":    "on",
		"is_active":          "on",
	})
	session.MakeRequest(t, req, http.StatusFound)
}

func TestLDAPUserSignin(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	prepareTestEnv(t)
	addAuthSourceLDAP(t)
	loginUserWithPassword(t, "fry", "fry")
}
