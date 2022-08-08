// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestEmailInvite(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user1")

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).(*organization.Organization)
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2}).(*organization.Team)

	req := NewRequestf(t, "GET", "/org/%s/teams/%s", org.Name, team.Name)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	url := fmt.Sprintf("/org/%s/teams/%s/action/add", org.Name, team.Name)

	req = NewRequestWithValues(t, "POST", url, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"uid":   "1",
		"uname": "user3@example.com",
	})

	resp = session.MakeRequest(t, req, http.StatusSeeOther)

	req = NewRequest(t, "GET", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invites, 1)

	user3Session := loginUser(t, "user3")

	// load the join page
	req = NewRequestf(t, "GET", "/org/invite/%s", invites[0].Token)
	resp = user3Session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	url = fmt.Sprintf("/org/invite/%s", invites[0].Token)

	// join the team
	req = NewRequestWithValues(t, "POST", url, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})

	resp = session.MakeRequest(t, req, http.StatusSeeOther)

	req = NewRequest(t, "GET", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusOK)
}
