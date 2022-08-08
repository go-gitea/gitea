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

func TestOrgTeamEmailInvite(t *testing.T) {
	defer prepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).(*organization.Organization)
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2}).(*organization.Team)

	session := loginUser(t, "user1")

	url := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	csrf := GetCSRF(t, session, url)
	req := NewRequestWithValues(t, "POST", url+"/action/add", map[string]string{
		"_csrf": csrf,
		"uid":   "1",
		"uname": "user5@example.com",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invites, 1)

	session = loginUser(t, "user5")

	// join the team
	url = fmt.Sprintf("/org/invite/%s", invites[0].Token)
	csrf = GetCSRF(t, session, url)
	req = NewRequestWithValues(t, "POST", url, map[string]string{
		"_csrf": csrf,
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)
}
