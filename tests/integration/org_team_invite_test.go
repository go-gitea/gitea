// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestOrgTeamEmailInvite(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember)

	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	csrf := GetCSRF(t, session, teamURL)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"_csrf": csrf,
		"uid":   "1",
		"uname": user.Email,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invites, 1)

	session = loginUser(t, user.Name)

	// join the team
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	csrf = GetCSRF(t, session, inviteURL)
	req = NewRequestWithValues(t, "POST", inviteURL, map[string]string{
		"_csrf": csrf,
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember)
}

// Check that users are redirected to accept the invitation correctly after login
func TestOrgTeamEmailInviteRedirectsExistingUser(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember)

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"_csrf": GetCSRF(t, session, teamURL),
		"uid":   "1",
		"uname": user.Email,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invites, 1)

	// accept the invite
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/login?redirect_to=%s", url.QueryEscape(inviteURL)))
	resp = MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"_csrf":     doc.GetCSRF(),
		"user_name": "user5",
		"password":  "password",
	})
	for _, c := range resp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	// complete the login process
	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session = emptyTestSession(t)
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	// make the request
	req = NewRequestWithValues(t, "POST", test.RedirectURL(resp), map[string]string{
		"_csrf": GetCSRF(t, session, test.RedirectURL(resp)),
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember)
}

// Check that newly signed up users are redirected to accept the invitation correctly
func TestOrgTeamEmailInviteRedirectsNewUser(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"_csrf": GetCSRF(t, session, teamURL),
		"uid":   "1",
		"uname": "doesnotexist@example.com",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invites, 1)

	// accept the invite
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/sign_up?redirect_to=%s", url.QueryEscape(inviteURL)))
	resp = MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"_csrf":     doc.GetCSRF(),
		"user_name": "doesnotexist",
		"email":     "doesnotexist@example.com",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	for _, c := range resp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	// complete the signup process
	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session = emptyTestSession(t)
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	// make the redirected request
	req = NewRequestWithValues(t, "POST", test.RedirectURL(resp), map[string]string{
		"_csrf": GetCSRF(t, session, test.RedirectURL(resp)),
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the new user
	newUser, err := user_model.GetUserByName(db.DefaultContext, "doesnotexist")
	assert.NoError(t, err)

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, newUser.ID)
	assert.NoError(t, err)
	assert.True(t, isMember)
}

// Check that users are redirected correctly after confirming their email
func TestOrgTeamEmailInviteRedirectsNewUserWithActivation(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	// enable email confirmation temporarily
	defer func(prevVal bool) {
		setting.Service.RegisterEmailConfirm = prevVal
	}(setting.Service.RegisterEmailConfirm)
	setting.Service.RegisterEmailConfirm = true

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"_csrf": GetCSRF(t, session, teamURL),
		"uid":   "1",
		"uname": "doesnotexist@example.com",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invites, 1)

	// accept the invite
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/sign_up?redirect_to=%s", url.QueryEscape(inviteURL)))
	inviteResp := MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"_csrf":     doc.GetCSRF(),
		"user_name": "doesnotexist",
		"email":     "doesnotexist@example.com",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	for _, c := range inviteResp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = MakeRequest(t, req, http.StatusOK)

	user, err := user_model.GetUserByName(db.DefaultContext, "doesnotexist")
	assert.NoError(t, err)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session = emptyTestSession(t)
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	activateURL := fmt.Sprintf("/user/activate?code=%s", user.GenerateEmailActivateCode("doesnotexist@example.com"))
	req = NewRequestWithValues(t, "POST", activateURL, map[string]string{
		"password": "examplePassword!1",
	})

	// use the cookies set by the signup request
	for _, c := range inviteResp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	// should be redirected to accept the invite
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	req = NewRequestWithValues(t, "POST", test.RedirectURL(resp), map[string]string{
		"_csrf": GetCSRF(t, session, test.RedirectURL(resp)),
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember)
}

// Test that a logged-in user who navigates to the sign-up link is then redirected using redirect_to
// For example: an invite may have been created before the user account was created, but they may be
// accepting the invite after having created an account separately
func TestOrgTeamEmailInviteRedirectsExistingUserWithLogin(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember)

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"_csrf": GetCSRF(t, session, teamURL),
		"uid":   "1",
		"uname": user.Email,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invites, 1)

	// note: the invited user has logged in
	session = loginUser(t, "user5")

	// accept the invite (note: this uses the sign_up url)
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/sign_up?redirect_to=%s", url.QueryEscape(inviteURL)))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	// make the request
	req = NewRequestWithValues(t, "POST", test.RedirectURL(resp), map[string]string{
		"_csrf": GetCSRF(t, session, test.RedirectURL(resp)),
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember)
}
