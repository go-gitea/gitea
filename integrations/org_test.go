// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrgRepos(t *testing.T) {
	prepareTestEnv(t)

	var (
		users = []string{"user1", "user2"}
		cases = map[string][]string{
			"alphabetically":        {"repo21", "repo3", "repo5"},
			"reversealphabetically": {"repo5", "repo3", "repo21"},
		}
	)

	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			session := loginUser(t, user)
			for sortBy, repos := range cases {
				req := NewRequest(t, "GET", "/user3?sort="+sortBy)
				resp := session.MakeRequest(t, req, http.StatusOK)

				htmlDoc := NewHTMLParser(t, resp.Body)

				sel := htmlDoc.doc.Find("a.name")
				assert.EqualValues(t, len(repos), len(sel.Nodes))
				for i := 0; i < len(repos); i++ {
					assert.EqualValues(t, repos[i], strings.TrimSpace(sel.Eq(i).Text()))
				}
			}
		})
	}
}

func TestLimitedOrg(t *testing.T) {
	prepareTestEnv(t)

	// not logged in user
	req := NewRequest(t, "GET", "/limited_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/limited_org/public_repo_on_limited_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/limited_org/private_repo_on_limited_org")
	MakeRequest(t, req, http.StatusNotFound)

	// login non-org member user
	session := loginUser(t, "user2")
	req = NewRequest(t, "GET", "/limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/public_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/private_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusNotFound)

	// site admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/public_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/private_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestPrivateOrg(t *testing.T) {
	prepareTestEnv(t)

	// not logged in user
	req := NewRequest(t, "GET", "/privated_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	MakeRequest(t, req, http.StatusNotFound)

	// login non-org member user
	session := loginUser(t, "user2")
	req = NewRequest(t, "GET", "/privated_org")
	session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusNotFound)

	// non-org member who is collaborator on repo in private org
	session = loginUser(t, "user4")
	req = NewRequest(t, "GET", "/privated_org")
	session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org") // colab of this repo
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusNotFound)

	// site admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/privated_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestOrgSettings(t *testing.T) {
	prepareTestEnv(t)

	type test struct {
		User   string
		Repo   string
		Result int
	}
	var (
		tests = []test{
			test{"user1", "user3", http.StatusOK},
			test{"user2", "user3", http.StatusOK},
			test{"user4", "user3", http.StatusNotFound},
			test{"user1", "limited_org", http.StatusOK},
			test{"user1", "privated_org", http.StatusOK},
			test{"user2", "limited_org", http.StatusNotFound},
			test{"user2", "privated_org", http.StatusNotFound},
		}
	)

	for _, te := range tests {
		t.Run(te.User+"/"+te.Repo, func(t *testing.T) {
			session := loginUser(t, te.User)
			req := NewRequest(t, "GET", "/org/"+te.Repo+"/settings")
			session.MakeRequest(t, req, te.Result)
			//resp := session.MakeRequest(t, req, http.StatusOK)
		})
	}
}

//TODO post a change

//TODO update the image

func TestOrgSettingsHooks(t *testing.T) {
	prepareTestEnv(t)

	type test struct {
		User   string
		Repo   string
		Result int
	}
	var (
		tests = []test{
			test{"user1", "user3", http.StatusOK},
			test{"user2", "user3", http.StatusOK},
			test{"user4", "user3", http.StatusNotFound},
			test{"user1", "limited_org", http.StatusOK},
			test{"user1", "privated_org", http.StatusOK},
			test{"user2", "limited_org", http.StatusNotFound},
			test{"user2", "privated_org", http.StatusNotFound},
		}
	)

	for _, te := range tests {
		t.Run(te.User+"/"+te.Repo, func(t *testing.T) {
			session := loginUser(t, te.User)
			req := NewRequest(t, "GET", "/org/"+te.Repo+"/settings/hooks")
			session.MakeRequest(t, req, te.Result)
			//resp := session.MakeRequest(t, req, http.StatusOK)
		})
	}
}

//TODO add webhook

func TestOrgSettingsDelete(t *testing.T) {
	prepareTestEnv(t)

	type test struct {
		User   string
		Repo   string
		Result int
	}
	var (
		tests = []test{
			test{"user1", "user3", http.StatusOK},
			test{"user2", "user3", http.StatusOK},
			test{"user4", "user3", http.StatusNotFound},
			test{"user1", "limited_org", http.StatusOK},
			test{"user1", "privated_org", http.StatusOK},
			test{"user2", "limited_org", http.StatusNotFound},
			test{"user2", "privated_org", http.StatusNotFound},
		}
	)

	for _, te := range tests {
		t.Run(te.User+"/"+te.Repo, func(t *testing.T) {
			session := loginUser(t, te.User)
			req := NewRequest(t, "GET", "/org/"+te.Repo+"/settings/delete")
			session.MakeRequest(t, req, te.Result)
			//resp := session.MakeRequest(t, req, http.StatusOK)
		})
	}
}

//TODO try delete
