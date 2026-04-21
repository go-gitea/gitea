// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/ldap"
	org_service "code.gitea.io/gitea/services/org"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ldapUser struct {
	UserName     string
	Password     string
	FullName     string
	Email        string
	OtherEmails  []string
	IsAdmin      bool
	IsRestricted bool
	SSHKeys      []string
}

type ldapTestEnv struct {
	gitLDAPUsers   []ldapUser
	otherLDAPUsers []ldapUser
	serverHost     string
	serverPort     string
}

func TestAuthLDAP(t *testing.T) {
	// To test it locally:
	// $ docker run --rm gitea/test-openldap:latest -p 389:389
	// $ TEST_LDAP=1 TEST_LDAP_HOST=localhost make "test-sqlite#TestAuthLDAP"
	defer tests.PrepareTestEnv(t)()
	t.Run("PreventInvalidGroupTeamMap", testLDAPPreventInvalidGroupTeamMap)
	t.Run("AuthChange", testLDAPAuthChange)
	t.Run("EmailSignin", testLDAPEmailSignin)

	hasRealServer, _ := strconv.ParseBool(os.Getenv("TEST_LDAP"))
	if hasRealServer {
		t.Run("UserSignin", testLDAPUserSignin)
		t.Run("UserSyncWithAttributeUsername", testLDAPUserSyncWithAttributeUsername)
		t.Run("UserSyncWithoutAttributeUsername", testLDAPUserSyncWithoutAttributeUsername)
		t.Run("UserSyncSSHKeys", testLDAPUserSyncSSHKeys)
		t.Run("UserSyncWithGroupFilter", testLDAPUserSyncWithGroupFilter)

		t.Run("GroupTeamSyncAddMember", testLDAPGroupTeamSyncAddMember)
		t.Run("GroupTeamSyncRemoveMember", testLDAPGroupTeamSyncRemoveMember)
	}
}

func prepareLdapTestServerEnv() *ldapTestEnv {
	gitLDAPUsers := []ldapUser{
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
			UserName:     "leela",
			Password:     "leela",
			FullName:     "Leela Turanga",
			Email:        "leela@planetexpress.com",
			IsRestricted: true,
		},
		{
			UserName: "bender",
			Password: "bender",
			FullName: "Bender Rodríguez",
			Email:    "bender@planetexpress.com",
		},
	}

	otherLDAPUsers := []ldapUser{
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

	return &ldapTestEnv{
		gitLDAPUsers:   gitLDAPUsers,
		otherLDAPUsers: otherLDAPUsers,
		serverHost:     util.IfZero(os.Getenv("TEST_LDAP_HOST"), "ldap"),
		serverPort:     util.IfZero(os.Getenv("TEST_LDAP_PORT"), "389"),
	}
}

func (te *ldapTestEnv) buildAuthSourcePayload(m map[string]string) map[string]string {
	ret := map[string]string{
		"type":                     "2",
		"name":                     "ldap",
		"host":                     te.serverHost,
		"port":                     te.serverPort,
		"bind_dn":                  "uid=gitea,ou=service,dc=planetexpress,dc=com",
		"bind_password":            "password",
		"user_base":                "ou=people,dc=planetexpress,dc=com",
		"filter":                   "(&(objectClass=inetOrgPerson)(memberOf=cn=git,ou=people,dc=planetexpress,dc=com)(uid=%s))",
		"admin_filter":             "(memberOf=cn=admin_staff,ou=people,dc=planetexpress,dc=com)",
		"restricted_filter":        "(uid=leela)",
		"attribute_username":       "uid",
		"attribute_name":           "givenName",
		"attribute_surname":        "sn",
		"attribute_mail":           "mail",
		"attribute_ssh_public_key": "",
		"is_sync_enabled":          "on",
		"is_active":                "on",
		"groups_enabled":           "on",
		"group_dn":                 "ou=people,dc=planetexpress,dc=com",
		"group_member_uid":         "member",
		"group_filter":             "",
		"group_team_map":           "",
		"group_team_map_removal":   "",
		"user_uid":                 "DN",
	}
	for k, v := range m {
		if _, ok := ret[k]; !ok {
			panic("invalid key: " + k)
		}
		ret[k] = v
	}
	return ret
}

func (te *ldapTestEnv) setupAuthSource(t *testing.T, params map[string]string) {
	session := loginUser(t, "user1")
	existing := &auth_model.Source{Name: params["name"]}
	if ok, _ := db.GetEngine(t.Context()).Get(existing); ok {
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/-/admin/auths/%d", existing.ID), params)
		session.MakeRequest(t, req, http.StatusSeeOther)
	} else {
		req := NewRequestWithValues(t, "POST", "/-/admin/auths/new", params)
		session.MakeRequest(t, req, http.StatusSeeOther)
	}
}

func testLDAPUserSignin(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := prepareLdapTestServerEnv()
	te.setupAuthSource(t, te.buildAuthSourcePayload(nil))

	t.Run("Success", func(t *testing.T) {
		u := te.gitLDAPUsers[0]
		session := loginUserWithPassword(t, u.UserName, u.Password)
		req := NewRequest(t, "GET", "/user/settings")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Equal(t, u.UserName, htmlDoc.GetInputValueByName("name"))
		assert.Equal(t, u.FullName, htmlDoc.GetInputValueByName("full_name"))
		assert.Equal(t, u.Email, htmlDoc.Find("#signed-user-email").Text())
	})
	t.Run("Failed", func(t *testing.T) {
		u := te.otherLDAPUsers[0]
		testLoginFailed(t, u.UserName, u.Password, translation.NewLocale("en-US").TrString("form.username_password_incorrect"))
	})
}

func testLDAPAuthChange(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := prepareLdapTestServerEnv()
	te.setupAuthSource(t, te.buildAuthSourcePayload(nil))

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/-/admin/auths")
	resp := session.MakeRequest(t, req, http.StatusOK)
	respStr := resp.Body.String()
	doc := NewHTMLParser(t, strings.NewReader(respStr))
	hrefAuthSource, exists := doc.Find("table.table td a").Attr("href")
	if !assert.True(t, exists, "No authentication source found") {
		t.Logf("response: %s", respStr)
		return
	}

	req = NewRequest(t, "GET", hrefAuthSource)
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)
	host, _ := doc.Find(`input[name="host"]`).Attr("value")
	assert.Equal(t, te.serverHost, host)
	bindDN, _ := doc.Find(`input[name="bind_dn"]`).Attr("value")
	assert.Equal(t, "uid=gitea,ou=service,dc=planetexpress,dc=com", bindDN)

	req = NewRequestWithValues(t, "POST", hrefAuthSource, te.buildAuthSourcePayload(map[string]string{"group_team_map_removal": "off"}))
	session.MakeRequest(t, req, http.StatusSeeOther)

	req = NewRequest(t, "GET", hrefAuthSource)
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)
	host, _ = doc.Find(`input[name="host"]`).Attr("value")
	assert.Equal(t, te.serverHost, host)
	bindDN, _ = doc.Find(`input[name="bind_dn"]`).Attr("value")
	assert.Equal(t, "uid=gitea,ou=service,dc=planetexpress,dc=com", bindDN)
}

func testLDAPUserSyncWithAttributeUsername(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := prepareLdapTestServerEnv()
	te.setupAuthSource(t, te.buildAuthSourcePayload(map[string]string{"attribute_username": "uid"}))

	// reset user and email table
	_, _ = db.GetEngine(t.Context()).Where("`name` != 'user1'").Delete(&user_model.User{})
	_ = db.TruncateBeans(t.Context(), &user_model.EmailAddress{})
	unittest.AssertCount(t, &user_model.User{}, 1)

	err := auth.SyncExternalUsers(t.Context(), true)
	require.NoError(t, err)

	// Check if users exists
	for _, gitLDAPUser := range te.gitLDAPUsers {
		dbUser, err := user_model.GetUserByName(t.Context(), gitLDAPUser.UserName)
		require.NoError(t, err)
		assert.Equal(t, gitLDAPUser.UserName, dbUser.Name)
		assert.Equal(t, gitLDAPUser.Email, dbUser.Email)
		assert.Equal(t, gitLDAPUser.IsAdmin, dbUser.IsAdmin)
		assert.Equal(t, gitLDAPUser.IsRestricted, dbUser.IsRestricted)
	}

	// Check if no users exist
	for _, otherLDAPUser := range te.otherLDAPUsers {
		_, err := user_model.GetUserByName(t.Context(), otherLDAPUser.UserName)
		assert.True(t, user_model.IsErrUserNotExist(err))
	}
}

func testLDAPUserSyncWithoutAttributeUsername(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := prepareLdapTestServerEnv()
	authParams := te.buildAuthSourcePayload(map[string]string{"attribute_username": ""})
	te.setupAuthSource(t, authParams)

	// reset user and email table
	_, _ = db.GetEngine(t.Context()).Where("`name` != 'user1'").Delete(&user_model.User{})
	_ = db.TruncateBeans(t.Context(), &user_model.EmailAddress{})
	unittest.AssertCount(t, &user_model.User{}, 1)

	adminSession := loginUser(t, "user1")
	for _, u := range te.gitLDAPUsers {
		req := NewRequest(t, "GET", "/-/admin/users?q="+u.UserName)
		resp := adminSession.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)

		tr := htmlDoc.doc.Find("table.table tbody tr:not(.no-results-row)")
		assert.Equal(t, 0, tr.Length())
	}

	for _, u := range te.gitLDAPUsers {
		req := NewRequestWithValues(t, "POST", "/user/login", map[string]string{
			"user_name": u.UserName,
			"password":  u.Password,
		})
		MakeRequest(t, req, http.StatusSeeOther)
	}

	require.NoError(t, auth.SyncExternalUsers(t.Context(), true))

	authSource := unittest.AssertExistsAndLoadBean(t, &auth_model.Source{Name: authParams["name"]})
	unittest.AssertCount(t, &user_model.User{
		LoginType:   auth_model.LDAP,
		LoginSource: authSource.ID,
	}, len(te.gitLDAPUsers))

	for _, u := range te.gitLDAPUsers {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name: u.UserName,
		})
		assert.True(t, user.IsActive)
	}
}

func testLDAPUserSyncWithGroupFilter(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := prepareLdapTestServerEnv()
	te.setupAuthSource(t, te.buildAuthSourcePayload(map[string]string{
		"filter":       "(&(objectClass=inetOrgPerson)(uid=%s))",
		"group_filter": "(cn=git)",
	}))

	// Assert a user not a member of the LDAP group "cn=git" cannot login
	// This test may look like TestLDAPUserSigninFailed but it is not.
	// The later test uses user filter containing group membership filter (memberOf)
	// This test is for the case when LDAP user records may not be linked with
	// all groups the user is a member of, the user filter is modified accordingly inside
	// the addAuthSourceLDAP based on the value of the groupFilter
	u := te.otherLDAPUsers[0]
	testLoginFailed(t, u.UserName, u.Password, translation.NewLocale("en-US").TrString("form.username_password_incorrect"))

	require.NoError(t, auth.SyncExternalUsers(t.Context(), true))

	// Assert members of LDAP group "cn=git" are added
	for _, gitLDAPUser := range te.gitLDAPUsers {
		unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name: gitLDAPUser.UserName,
		})
	}

	// Assert everyone else is not added
	for _, gitLDAPUser := range te.otherLDAPUsers {
		unittest.AssertNotExistsBean(t, &user_model.User{
			Name: gitLDAPUser.UserName,
		})
	}

	ldapSource := unittest.AssertExistsAndLoadBean(t, &auth_model.Source{
		Name: "ldap",
	})
	ldapConfig := ldapSource.Cfg.(*ldap.Source)
	ldapConfig.GroupFilter = "(cn=ship_crew)"
	require.NoError(t, auth_model.UpdateSource(t.Context(), ldapSource))

	require.NoError(t, auth.SyncExternalUsers(t.Context(), true))

	for _, gitLDAPUser := range te.gitLDAPUsers {
		if gitLDAPUser.UserName == "fry" || gitLDAPUser.UserName == "leela" || gitLDAPUser.UserName == "bender" {
			// Assert members of the LDAP group "cn-ship_crew" are still active
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{
				Name: gitLDAPUser.UserName,
			})
			assert.True(t, user.IsActive, "User %s should be active", gitLDAPUser.UserName)
		} else {
			// Assert everyone else is inactive
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{
				Name: gitLDAPUser.UserName,
			})
			assert.False(t, user.IsActive, "User %s should be inactive", gitLDAPUser.UserName)
		}
	}
}

func testLDAPUserSyncSSHKeys(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := prepareLdapTestServerEnv()
	te.setupAuthSource(t, te.buildAuthSourcePayload(map[string]string{"attribute_ssh_public_key": "sshPublicKey"}))

	require.NoError(t, auth.SyncExternalUsers(t.Context(), true))

	// Check if users has SSH keys synced
	count := 0
	for _, u := range te.gitLDAPUsers {
		if len(u.SSHKeys) == 0 {
			continue
		}
		count++

		session := loginUserWithPassword(t, u.UserName, u.Password)
		req := NewRequest(t, "GET", "/user/settings/keys")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		divs := htmlDoc.doc.Find("#keys-ssh .flex-item .flex-item-body:not(:last-child)")
		syncedKeys := make([]string, divs.Length())
		for i := 0; i < divs.Length(); i++ {
			syncedKeys[i] = strings.TrimSpace(divs.Eq(i).Text())
		}
		assert.ElementsMatch(t, u.SSHKeys, syncedKeys, "Unequal number of keys synchronized for user: %s", u.UserName)
	}
	assert.NotZero(t, count)
}

func testLDAPGroupTeamSyncAddMember(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	te := prepareLdapTestServerEnv()
	te.setupAuthSource(t, te.buildAuthSourcePayload(map[string]string{
		"group_team_map":         `{"cn=ship_crew,ou=people,dc=planetexpress,dc=com":{"org26": ["team11"]},"cn=admin_staff,ou=people,dc=planetexpress,dc=com": {"non-existent": ["non-existent"]}}`,
		"group_team_map_removal": "on",
	}))
	org, err := organization.GetOrgByName(t.Context(), "org26")
	assert.NoError(t, err)
	team, err := organization.GetTeam(t.Context(), org.ID, "team11")
	assert.NoError(t, err)
	require.NoError(t, auth.SyncExternalUsers(t.Context(), true))
	for _, gitLDAPUser := range te.gitLDAPUsers {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name: gitLDAPUser.UserName,
		})
		usersOrgs, err := db.Find[organization.Organization](t.Context(), organization.FindOrgOptions{
			UserID:            user.ID,
			IncludeVisibility: structs.VisibleTypePrivate,
		})
		assert.NoError(t, err)
		allOrgTeams, err := organization.GetUserOrgTeams(t.Context(), org.ID, user.ID)
		assert.NoError(t, err)
		if user.Name == "fry" || user.Name == "leela" || user.Name == "bender" {
			// assert members of LDAP group "cn=ship_crew" are added to mapped teams
			assert.Len(t, usersOrgs, 1, "User [%s] should be member of one organization", user.Name)
			assert.Equal(t, "org26", usersOrgs[0].Name, "Membership should be added to the right organization")
			isMember, err := organization.IsTeamMember(t.Context(), usersOrgs[0].ID, team.ID, user.ID)
			assert.NoError(t, err)
			assert.True(t, isMember, "Membership should be added to the right team")
			err = org_service.RemoveTeamMember(t.Context(), team, user)
			assert.NoError(t, err)
			err = org_service.RemoveOrgUser(t.Context(), usersOrgs[0], user)
			assert.NoError(t, err)
		} else {
			// assert members of LDAP group "cn=admin_staff" keep initial team membership since mapped team does not exist
			assert.Empty(t, usersOrgs, "User should be member of no organization")
			isMember, err := organization.IsTeamMember(t.Context(), org.ID, team.ID, user.ID)
			assert.NoError(t, err)
			assert.False(t, isMember, "User should no be added to this team")
			assert.Empty(t, allOrgTeams, "User should not be added to any team")
		}
	}
}

func testLDAPGroupTeamSyncRemoveMember(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	te := prepareLdapTestServerEnv()
	te.setupAuthSource(t, te.buildAuthSourcePayload(map[string]string{
		"group_team_map":         `{"cn=dispatch,ou=people,dc=planetexpress,dc=com": {"org26": ["team11"]}}`,
		"group_team_map_removal": "on",
	}))

	org, err := organization.GetOrgByName(t.Context(), "org26")
	assert.NoError(t, err)
	team, err := organization.GetTeam(t.Context(), org.ID, "team11")
	assert.NoError(t, err)
	loginUserWithPassword(t, te.gitLDAPUsers[0].UserName, te.gitLDAPUsers[0].Password)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		Name: te.gitLDAPUsers[0].UserName,
	})
	err = organization.AddOrgUser(t.Context(), org.ID, user.ID)
	assert.NoError(t, err)
	err = org_service.AddTeamMember(t.Context(), team, user)
	assert.NoError(t, err)
	isMember, err := organization.IsOrganizationMember(t.Context(), org.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember, "User should be member of this organization")
	isMember, err = organization.IsTeamMember(t.Context(), org.ID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember, "User should be member of this team")
	// assert team member "professor" gets removed from org26 team11
	loginUserWithPassword(t, te.gitLDAPUsers[0].UserName, te.gitLDAPUsers[0].Password)
	isMember, err = organization.IsOrganizationMember(t.Context(), org.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember, "User membership should have been removed from organization")
	isMember, err = organization.IsTeamMember(t.Context(), org.ID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember, "User membership should have been removed from team")
}

func testLDAPPreventInvalidGroupTeamMap(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := prepareLdapTestServerEnv()

	session := loginUser(t, "user1")
	payload := te.buildAuthSourcePayload(map[string]string{"group_team_map": `{"NOT_A_VALID_JSON"["MISSING_DOUBLE_POINT"]}`, "group_team_map_removal": "off"})
	req := NewRequestWithValues(t, "POST", "/-/admin/auths/new", payload)
	session.MakeRequest(t, req, http.StatusOK) // StatusOK = failed, StatusSeeOther = ok
}

func testLDAPEmailSignin(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	te := ldapTestEnv{
		gitLDAPUsers: []ldapUser{
			{
				UserName: "u1",
				Password: "xx",
				FullName: "user 1",
				Email:    "u1@gitea.com",
			},
		},
		serverHost: "mock-host",
		serverPort: "mock-port",
	}
	defer test.MockVariableValue(&ldap.MockedSearchEntry, func(source *ldap.Source, name, passwd string, directBind bool) *ldap.SearchResult {
		var u *ldapUser
		for _, user := range te.gitLDAPUsers {
			if user.Email == name && user.Password == passwd {
				u = &user
				break
			}
		}
		if u == nil {
			return nil
		}
		result := &ldap.SearchResult{
			Username:  u.UserName,
			Mail:      u.Email,
			LowerName: strings.ToLower(u.UserName),
		}
		nameFields := strings.Split(u.FullName, " ")
		result.Name = nameFields[0]
		if len(nameFields) > 1 {
			result.Surname = nameFields[1]
		}
		return result
	})()
	defer tests.PrepareTestEnv(t)()
	te.setupAuthSource(t, te.buildAuthSourcePayload(nil))

	u := te.gitLDAPUsers[0]

	session := loginUserWithPassword(t, u.Email, u.Password)
	req := NewRequest(t, "GET", "/user/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	assert.Equal(t, u.UserName, htmlDoc.GetInputValueByName("name"))
	assert.Equal(t, u.FullName, htmlDoc.GetInputValueByName("full_name"))
	assert.Equal(t, u.Email, htmlDoc.Find("#signed-user-email").Text())
}
