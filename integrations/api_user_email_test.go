// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIListEmails(t *testing.T) {
	defer prepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session)

	req := NewRequest(t, "GET", "/api/v1/user/emails?token="+token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var emails []*api.Email
	DecodeJSON(t, resp, &emails)

	assert.EqualValues(t, []*api.Email{
		{
			Email:    "user2@example.com",
			Verified: true,
			Primary:  true,
		},
		{
			Email:    "user21@example.com",
			Verified: false,
			Primary:  false,
		},
	}, emails)
}

func TestAPIAddEmail(t *testing.T) {
	defer prepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session)

	opts := api.CreateEmailOption{
		Emails: []string{"user101@example.com"},
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/emails?token="+token, &opts)
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	opts = api.CreateEmailOption{
		Emails: []string{"user22@example.com"},
	}
	req = NewRequestWithJSON(t, "POST", "/api/v1/user/emails?token="+token, &opts)
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var emails []*api.Email
	DecodeJSON(t, resp, &emails)
	assert.EqualValues(t, []*api.Email{
		{
			Email:    "user22@example.com",
			Verified: true,
			Primary:  false,
		},
	}, emails)
}

func TestAPIDeleteEmail(t *testing.T) {
	defer prepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session)

	opts := api.DeleteEmailOption{
		Emails: []string{"user22@example.com"},
	}
	req := NewRequestWithJSON(t, "DELETE", "/api/v1/user/emails?token="+token, &opts)
	session.MakeRequest(t, req, http.StatusNotFound)

	opts = api.DeleteEmailOption{
		Emails: []string{"user21@example.com"},
	}
	req = NewRequestWithJSON(t, "DELETE", "/api/v1/user/emails?token="+token, &opts)
	session.MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "GET", "/api/v1/user/emails?token="+token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var emails []*api.Email
	DecodeJSON(t, resp, &emails)
	assert.EqualValues(t, []*api.Email{
		{
			Email:    "user2@example.com",
			Verified: true,
			Primary:  true,
		},
	}, emails)
}
