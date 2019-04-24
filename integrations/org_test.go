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
