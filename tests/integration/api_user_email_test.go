// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

// TestAPIManageEmailsFeatureDisabled ensures the email management API honors the
// manage_credentials feature restriction, matching the web UI.
func TestAPIManageEmailsFeatureDisabled(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	WithDisabledFeatures(t, setting.UserFeatureManageCredentials)

	addReq := NewRequestWithJSON(t, "POST", "/api/v1/user/emails", &api.CreateEmailOption{
		Emails: []string{"user2-3@example.com"},
	}).AddTokenAuth(token)
	MakeRequest(t, addReq, http.StatusNotFound)

	delReq := NewRequestWithJSON(t, "DELETE", "/api/v1/user/emails", &api.DeleteEmailOption{
		Emails: []string{"user2-2@example.com"},
	}).AddTokenAuth(token)
	MakeRequest(t, delReq, http.StatusNotFound)
}

func TestAPIListEmails(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	req := NewRequest(t, "GET", "/api/v1/user/emails").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	emails := DecodeJSON(t, resp, []*api.Email{})

	assert.Equal(t, []*api.Email{
		{
			Email:    "user2@example.com",
			Verified: true,
			Primary:  true,
		},
		{
			Email:    "user2-2@example.com",
			Verified: false,
			Primary:  false,
		},
	}, emails)
}

func TestAPIAddEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	opts := api.CreateEmailOption{
		Emails: []string{"user101@example.com"},
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/emails", &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	opts = api.CreateEmailOption{
		Emails: []string{"user2-3@example.com"},
	}
	req = NewRequestWithJSON(t, "POST", "/api/v1/user/emails", &opts).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	emails := DecodeJSON(t, resp, []*api.Email{})
	assert.Equal(t, []*api.Email{
		{
			Email:    "user2@example.com",
			Verified: true,
			Primary:  true,
		},
		{
			Email:    "user2-2@example.com",
			Verified: false,
			Primary:  false,
		},
		{
			Email:    "user2-3@example.com",
			Verified: true,
			Primary:  false,
		},
	}, emails)

	opts = api.CreateEmailOption{
		Emails: []string{"notAEmail"},
	}
	req = NewRequestWithJSON(t, "POST", "/api/v1/user/emails", &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPIDeleteEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	opts := api.DeleteEmailOption{
		Emails: []string{"user2-3@example.com"},
	}
	req := NewRequestWithJSON(t, "DELETE", "/api/v1/user/emails", &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	opts = api.DeleteEmailOption{
		Emails: []string{"user2-2@example.com"},
	}
	req = NewRequestWithJSON(t, "DELETE", "/api/v1/user/emails", &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "GET", "/api/v1/user/emails").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	emails := DecodeJSON(t, resp, []*api.Email{})
	assert.Equal(t, []*api.Email{
		{
			Email:    "user2@example.com",
			Verified: true,
			Primary:  true,
		},
	}, emails)
}
