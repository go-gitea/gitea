// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
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
			{"user1", "user3", http.StatusOK},
			{"user2", "user3", http.StatusOK},
			{"user4", "user3", http.StatusNotFound},
			{"user1", "limited_org", http.StatusOK},
			{"user1", "privated_org", http.StatusOK},
			{"user2", "limited_org", http.StatusNotFound},
			{"user2", "privated_org", http.StatusNotFound},
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
			{"user1", "user3", http.StatusOK},
			{"user2", "user3", http.StatusOK},
			{"user4", "user3", http.StatusNotFound},
			{"user1", "limited_org", http.StatusOK},
			{"user1", "privated_org", http.StatusOK},
			{"user2", "limited_org", http.StatusNotFound},
			{"user2", "privated_org", http.StatusNotFound},
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
func TestOrgSettingsHooksAdd(t *testing.T) {
	prepareTestEnv(t)

	type test struct {
		User            string
		Repo            string
		Type            string
		GetResult       int
		HookData        map[string]string
		PostResult      int
		HookCountBefore int
		DeleteResult    int
	}
	var (
		tests = []test{
			{"user1", "privated_org", "gitea", http.StatusOK, map[string]string{
				"payload_url":  "http://localhost:8080",
				"content_type": "1",
				"secret":       "some_secret",
				"events":       "send_everything",
				"active":       "on",
			}, http.StatusFound, 0, http.StatusOK},
		}
	)

	for _, te := range tests {
		t.Run(te.User+"/"+te.Repo, func(t *testing.T) {
			session := loginUser(t, te.User)
			req := NewRequest(t, "GET", "/org/"+te.Repo+"/settings/hooks/"+te.Type+"/new")
			resp := session.MakeRequest(t, req, te.GetResult)

			//t.Logf("debug: %s", resp.Body)
			htmlDoc := NewHTMLParser(t, resp.Body)
			sel := htmlDoc.doc.Find("a.dont-break-out")
			assert.EqualValues(t, te.HookCountBefore, len(sel.Nodes))

			te.HookData["_csrf"] = GetCSRF(t, session, "/org/"+te.Repo+"/settings/hooks/"+te.Type+"/new")
			req = NewRequestWithValues(t, "POST", "/org/"+te.Repo+"/settings/hooks/"+te.Type+"/new", te.HookData)
			session.MakeRequest(t, req, te.PostResult)

			req = NewRequest(t, "GET", fmt.Sprintf("/org/%s/settings/hooks", te.Repo))
			resp = session.MakeRequest(t, req, te.GetResult)
			session.MakeRequest(t, req, te.GetResult)
			//t.Logf("debug: %s", resp.Body)
			htmlDoc = NewHTMLParser(t, resp.Body)
			sel = htmlDoc.doc.Find("a.delete-button")
			assert.EqualValues(t, te.HookCountBefore+1, len(sel.Nodes))
			hookEl := sel.Nodes[len(sel.Nodes)-1]
			hookID := "0"
			for _, a := range hookEl.Attr {
				if a.Key == "data-id" {
					hookID = a.Val
				}
			}

			req = NewRequest(t, "GET", fmt.Sprintf("/org/%s/settings/hooks/%s", te.Repo, hookID))
			resp = session.MakeRequest(t, req, te.GetResult)
			htmlDoc = NewHTMLParser(t, resp.Body)

			//t.Logf("debug: %s", htmlDoc.GetCSRF())
			req = NewRequestWithValues(t, "POST", "/org/"+te.Repo+"/settings/hooks/delete", map[string]string{
				"_csrf": htmlDoc.GetCSRF(),
				"id":    hookID,
			})
			session.MakeRequest(t, req, te.DeleteResult)
		})
	}
}

func TestOrgSettingsDelete(t *testing.T) {
	prepareTestEnv(t)

	type test struct {
		User   string
		Repo   string
		Result int
		Delete bool
	}
	var (
		tests = []test{
			{"user1", "user3", http.StatusOK, false},
			{"user2", "user3", http.StatusOK, false},
			{"user4", "user3", http.StatusNotFound, false},
			{"user1", "limited_org", http.StatusOK, false},
			{"user1", "privated_org", http.StatusOK, false},
			{"user2", "limited_org", http.StatusNotFound, false},
			{"user2", "privated_org", http.StatusNotFound, false},
			{"user1", "privated_org", http.StatusOK, true},
		}
	)

	for _, te := range tests {
		t.Run(te.User+"/"+te.Repo, func(t *testing.T) {
			session := loginUser(t, te.User)
			req := NewRequest(t, "GET", "/org/"+te.Repo+"/settings/delete")
			resp := session.MakeRequest(t, req, te.Result)

			if te.Delete {
				htmlDoc := NewHTMLParser(t, resp.Body)
				req := NewRequestWithValues(t, "POST", "/org/"+te.Repo+"/settings/delete", map[string]string{
					"_csrf":    htmlDoc.GetCSRF(),
					"password": "password",
				})
				session.MakeRequest(t, req, http.StatusFound)
			}
		})
	}
}
