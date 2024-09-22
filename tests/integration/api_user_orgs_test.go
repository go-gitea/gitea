// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestUserOrgs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	privateMemberUsername := "user4"
	unrelatedUsername := "user5"

	orgs := getUserOrgs(t, adminUsername, normalUsername)

	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "org3"})
	org17 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "org17"})
	org35 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "private_org35"})

	assert.Equal(t, []*api.Organization{
		{
			ID:          17,
			Name:        org17.Name,
			UserName:    org17.Name,
			FullName:    org17.FullName,
			Email:       org17.Email,
			AvatarURL:   org17.AvatarLink(db.DefaultContext),
			Description: "",
			Website:     "",
			Location:    "",
			Visibility:  "public",
		},
		{
			ID:          3,
			Name:        org3.Name,
			UserName:    org3.Name,
			FullName:    org3.FullName,
			Email:       org3.Email,
			AvatarURL:   org3.AvatarLink(db.DefaultContext),
			Description: "",
			Website:     "",
			Location:    "",
			Visibility:  "public",
		},
		{
			ID:          35,
			Name:        org35.Name,
			UserName:    org35.Name,
			FullName:    org35.FullName,
			Email:       org35.Email,
			AvatarURL:   org35.AvatarLink(db.DefaultContext),
			Description: "",
			Website:     "",
			Location:    "",
			Visibility:  "private",
		},
	}, orgs)

	// user itself should get it's org's he is a member of
	orgs = getUserOrgs(t, privateMemberUsername, privateMemberUsername)
	assert.Len(t, orgs, 1)

	// unrelated user should not get private org membership of privateMemberUsername
	orgs = getUserOrgs(t, unrelatedUsername, privateMemberUsername)
	assert.Len(t, orgs, 0)

	// not authenticated call should not be allowed
	testUserOrgsUnauthenticated(t, privateMemberUsername)
}

func getUserOrgs(t *testing.T, userDoer, userCheck string) (orgs []*api.Organization) {
	token := ""
	if len(userDoer) != 0 {
		token = getUserToken(t, userDoer, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadUser)
	}
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/orgs", userCheck)).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &orgs)
	return orgs
}

func testUserOrgsUnauthenticated(t *testing.T, userCheck string) {
	session := emptyTestSession(t)
	req := NewRequestf(t, "GET", "/api/v1/users/%s/orgs", userCheck)
	session.MakeRequest(t, req, http.StatusUnauthorized)
}

func TestMyOrgs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/user/orgs")
	MakeRequest(t, req, http.StatusUnauthorized)

	normalUsername := "user2"
	token := getUserToken(t, normalUsername, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadUser)
	req = NewRequest(t, "GET", "/api/v1/user/orgs").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var orgs []*api.Organization
	DecodeJSON(t, resp, &orgs)
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "org3"})
	org17 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "org17"})
	org35 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "private_org35"})

	assert.Equal(t, []*api.Organization{
		{
			ID:          17,
			Name:        org17.Name,
			UserName:    org17.Name,
			FullName:    org17.FullName,
			Email:       org17.Email,
			AvatarURL:   org17.AvatarLink(db.DefaultContext),
			Description: "",
			Website:     "",
			Location:    "",
			Visibility:  "public",
		},
		{
			ID:          3,
			Name:        org3.Name,
			UserName:    org3.Name,
			FullName:    org3.FullName,
			Email:       org3.Email,
			AvatarURL:   org3.AvatarLink(db.DefaultContext),
			Description: "",
			Website:     "",
			Location:    "",
			Visibility:  "public",
		},
		{
			ID:          35,
			Name:        org35.Name,
			UserName:    org35.Name,
			FullName:    org35.FullName,
			Email:       org35.Email,
			AvatarURL:   org35.AvatarLink(db.DefaultContext),
			Description: "",
			Website:     "",
			Location:    "",
			Visibility:  "private",
		},
	}, orgs)
}
