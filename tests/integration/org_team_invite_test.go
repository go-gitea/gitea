// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
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

	url := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	csrf := GetCSRF(t, session, url)
	req := NewRequestWithValues(t, "POST", url+"/action/add", map[string]string{
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
	url = fmt.Sprintf("/org/invite/%s", invites[0].Token)
	csrf = GetCSRF(t, session, url)
	req = NewRequestWithValues(t, "POST", url, map[string]string{
		"_csrf": csrf,
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember)
}
