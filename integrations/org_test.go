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
			{"user1", "privated_org", "gogs", http.StatusOK, map[string]string{
				"payload_url":  "http://localhost:8080",
				"content_type": "1",
				"secret":       "some_secret",
				"events":       "send_everything",
				"active":       "on",
			}, http.StatusFound, 0, http.StatusOK},
			{"user1", "privated_org", "slack", http.StatusOK, map[string]string{
				"payload_url": "http://localhost:8080",
				"channel":     "#test",
				"username":    "gitea",
				"icon_url":    "",
				"color":       "",
				"events":      "send_everything",
				"active":      "on",
			}, http.StatusFound, 0, http.StatusOK},
			{"user1", "privated_org", "discord", http.StatusOK, map[string]string{
				"payload_url": "http://localhost:8080",
				"username":    "gitea",
				"icon_url":    "",
				"events":      "send_everything",
				"active":      "on",
			}, http.StatusFound, 0, http.StatusOK},
			{"user1", "privated_org", "dingtalk", http.StatusOK, map[string]string{
				"payload_url": "http://localhost:8080",
				"events":      "send_everything",
				"active":      "on",
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
			req := NewRequest(t, "GET", "/org/"+te.Repo+"/settings/delete")
			session.MakeRequest(t, req, te.Result)
		})
	}
}

func TestOrgSettingsCreateAndDelete(t *testing.T) {
	session := loginUser(t, "user1")

	//Exist user
	req := NewRequest(t, "GET", "/org/create")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/create", map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"org_name":   "user1",
		"visibility": "0",
	})
	session.MakeRequest(t, req, http.StatusBadRequest)

	//Exist org
	req = NewRequest(t, "GET", "/org/create")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/create", map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"org_name":   "limited_org",
		"visibility": "0",
	})
	session.MakeRequest(t, req, http.StatusBadRequest)

	//Restricted
	req = NewRequest(t, "GET", "/org/create")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/create", map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"org_name":   "assets",
		"visibility": "0",
	})
	session.MakeRequest(t, req, http.StatusBadRequest)

	//Restricted pattern
	req = NewRequest(t, "GET", "/org/create")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/create", map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"org_name":   "user.gpg",
		"visibility": "0",
	})
	session.MakeRequest(t, req, http.StatusBadRequest)

	//Forbidden
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/org/create")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/create", map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"org_name":   "test_org_to_delete",
		"visibility": "0",
	})
	session.MakeRequest(t, req, http.StatusBadRequest)

	//OK
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/org/create")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/create", map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"org_name":   "test_org_to_delete",
		"visibility": "0",
	})
	session.MakeRequest(t, req, http.StatusFound)

	//Update desc
	req = NewRequest(t, "GET", "/org/test_org_to_delete/settings")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/test_org_to_delete/settings", map[string]string{
		"_csrf":             htmlDoc.GetCSRF(),
		"name":              "test_org_to_delete",
		"full_name":         "test_org_to_delete",
		"description":       "Some little desc",
		"website":           "",
		"location":          "",
		"visibility":        "0",
		"max_repo_creation": "-1",
	})
	session.MakeRequest(t, req, http.StatusFound)

	req = NewRequest(t, "GET", "/org/test_org_to_delete/settings")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/test_org_to_delete/settings/delete", map[string]string{
		"_csrf":    htmlDoc.GetCSRF(),
		"password": "wrong_password",
	})
	resp = session.MakeRequest(t, req, http.StatusOK) //Maybe should not be OK ?
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "div.ui.negative.message", true)
	//The password you entered is incorrect.

	req = NewRequest(t, "GET", "/org/test_org_to_delete/settings")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/test_org_to_delete/settings/delete", map[string]string{
		"_csrf":    htmlDoc.GetCSRF(),
		"password": "password",
	})
	resp = session.MakeRequest(t, req, http.StatusFound)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "div.ui.negative.message", false)
}

func TestOrgTeam(t *testing.T) {
	org := "user3"
	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/org/"+org+"/teams")
	session.MakeRequest(t, req, http.StatusOK) //TODO count teams

	req = NewRequest(t, "GET", "/org/"+org+"/teams/new")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/"+org+"/teams/new", map[string]string{
		"_csrf":       htmlDoc.GetCSRF(),
		"team_name":   "team_test",
		"description": "team_test_desc",
		"permission":  "admin",
	})
	session.MakeRequest(t, req, http.StatusFound)

	req = NewRequest(t, "GET", "/org/"+org+"/teams/team_test")
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/"+org+"/teams/team_test/action/add", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"uid":   "2",
		"uname": "user2",
	})
	session.MakeRequest(t, req, http.StatusFound)

	req = NewRequest(t, "GET", "/org/"+org+"/teams/team_test/action/remove?uid=2") //TODO should be POST
	session.MakeRequest(t, req, http.StatusFound)

	req = NewRequest(t, "GET", "/org/"+org+"/teams/team_test/repositories")
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/org/"+org+"/teams/team_test/edit")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/"+org+"/teams/team_test/edit", map[string]string{
		"_csrf":       htmlDoc.GetCSRF(),
		"team_name":   "team_test",
		"description": "team_test_desc_2",
	})
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/org/"+org+"/teams/team_test/edit")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	req = NewRequestWithValues(t, "POST", "/org/"+org+"/teams/team_test/delete", map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	session.MakeRequest(t, req, http.StatusOK) //TODO should be StatusFound
}

func TestOrgMembers(t *testing.T) {
	org := "user3"
	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/org/"+org+"/members")
	session.MakeRequest(t, req, http.StatusOK) //TODO count members

	req = NewRequest(t, "GET", "/org/"+org+"/members/action/private?uid=1")
	session.MakeRequest(t, req, http.StatusFound) //TODO should use POST //TODO count members

	req = NewRequest(t, "GET", "/org/"+org+"/members/action/public?uid=1")
	session.MakeRequest(t, req, http.StatusFound) //TODO should use POST //TODO count members

	req = NewRequest(t, "GET", "/org/"+org+"/members/action/private?uid=2")
	session.MakeRequest(t, req, http.StatusFound) //TODO should use POST //TODO count members
	req = NewRequest(t, "GET", "/org/"+org+"/members/action/remove?uid=2")
	session.MakeRequest(t, req, http.StatusFound) //TODO should use POST //TODO count members
	req = NewRequest(t, "GET", "/org/"+org+"/members/action/public?uid=2")
	session.MakeRequest(t, req, http.StatusFound) //TODO should use POST //TODO count members

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/org/"+org+"/members/action/leave")
	session.MakeRequest(t, req, http.StatusFound) //TODO should use POST //TODO count members

}
