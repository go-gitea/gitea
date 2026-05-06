// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	perm_model "code.gitea.io/gitea/models/perm"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
	"xorm.io/builder"

	"github.com/stretchr/testify/assert"
)

func seedOrgWithGroups(t *testing.T) {
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization)
	const orgName = "org-with-groups"
	baseOrgUrl := fmt.Sprintf("/api/v1/orgs/%s", orgName)

	org := api.CreateOrgOption{
		UserName:    orgName,
		FullName:    "Org with groups",
		Description: "This organization has subgroups",
		Website:     "https://try.gitea.io",
		Location:    "Shanghai",
		Visibility:  "public",
	}
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &org).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	apiOrg := DecodeJSON(t, resp, &api.Organization{})

	e := db.GetEngine(t.Context())

	_, err := e.Table(&group_model.Group{}).Update(&group_model.Group{
		OwnerName: apiOrg.Name,
		OwnerID:   apiOrg.ID,
	})
	assert.NoError(t, err)

	// seed teams
	teamPrivs := map[string]perm_model.AccessMode{
		"Owners":  perm_model.AccessModeOwner,
		"Admins":  perm_model.AccessModeAdmin,
		"Writers": perm_model.AccessModeWrite,
		"Readers": perm_model.AccessModeRead,
		"Limited": perm_model.AccessModeNone,
	}
	var teams []*api.Team

	userIds := []int64{4, 5, 8, 9, 10}

	userIdIdx := 0
	for k, v := range teamPrivs {
		perm := api.RepoWritePermissionRead
		if v >= perm_model.AccessModeAdmin {
			perm = api.RepoWritePermissionAdmin
		} else if v >= perm_model.AccessModeWrite {
			perm = api.RepoWritePermissionWrite
		}
		reqBody := &api.CreateTeamOption{
			Name:                    k,
			CanCreateOrgRepo:        v >= perm_model.AccessModeWrite,
			UnitsMap:                map[string]string{},
			IncludesAllRepositories: v >= perm_model.AccessModeWrite,
		}
		if v > perm_model.AccessModeNone {
			reqBody.Permission = perm
		}
		for _, nunit := range unit_model.AllUnitKeyNames() {
			reqBody.UnitsMap[nunit] = v.ToString()
		}
		treq := NewRequestWithJSON(t, "POST", baseOrgUrl+"/teams", reqBody).AddTokenAuth(token)
		tresp := MakeRequest(t, treq, http.StatusCreated)
		team := DecodeJSON(t, tresp, &api.Team{})
		teams = append(teams, team)

		teamUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userIds[userIdIdx]})

		mreq := NewRequestf(t, "PUT", "/teams/%d/members/%s", team.ID, teamUser.Name)
		MakeRequest(t, mreq, http.StatusNoContent)
		userIdIdx++
	}
	allPrivateGroups, err := group_model.FindGroupsByCond(t.Context(), &group_model.FindGroupsOptions{
		OwnerID: apiOrg.ID,
	}, builder.Eq{"`repo_group`.visibility": api.VisibleTypePrivate})
	assert.NoError(t, err)

	for _, group := range allPrivateGroups {
		baseTeamUrl := fmt.Sprintf("/api/v1/groups/%d/teams", group.ID)
		for _, team := range teams {
			trq := NewRequestWithJSON(t, "PUT", baseTeamUrl+"/"+team.Name, &api.CreateOrUpdateRepoGroupTeamOption{
				CanCreateIn: new(group.ID%int64(2) == int64(0) && perm_model.ParseAccessMode(string(team.Permission)) > perm_model.AccessModeRead),
			}).AddTokenAuth(token)
			MakeRequest(t, trq, http.StatusNoContent)
			assert.NoError(t, err)
		}
	}
}

func getOrgWithGroups(t *testing.T) *api.Organization {
	req := NewRequest(t, "GET", "/api/v1/orgs/org-with-groups")
	resp := MakeRequest(t, req, http.StatusOK)
	return DecodeJSON(t, resp, &api.Organization{})
}

func TestAPIGroup(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	seedOrgWithGroups(t)
	t.Run("Visibility", func(t *testing.T) {
		t.Run("OwnersAndSiteAdminsCanSeeAllTopLevelGroups", testOwnersAndAdminsCanSeeAllTopLevelGroups)
		t.Run("NonOrgMemberWontSeeHiddenTopLevelGroups", testNonOrgMemberWontSeeHiddenTopLevelGroups)
	})
}

func TestCreateGroup(t *testing.T) {
}

func testOwnersAndAdminsCanSeeAllTopLevelGroups(t *testing.T) {
	org := getOrgWithGroups(t)
	req := NewRequestf(t, "GET", "/api/v1/orgs/org-with-groups/groups").AddBasicAuth("user2")
	resp := MakeRequest(t, req, http.StatusOK)
	groups := DecodeJSON(t, resp, []api.Group{})
	expectedLen := unittest.GetCount(t, new(group_model.Group),
		group_model.FindGroupsOptions{
			ParentGroupID: 0,
			OwnerID:       org.ID,
		}.ToConds())
	assert.Len(t, groups, expectedLen)

	// now test if site-wide admin can see all groups
	req = NewRequestf(t, "GET", "/api/v1/orgs/org-with-groups/groups").AddBasicAuth("user1")
	resp = MakeRequest(t, req, http.StatusOK)
	groups = DecodeJSON(t, resp, []api.Group{})
	assert.Len(t, groups, expectedLen)
}

func testNonOrgMemberWontSeeHiddenTopLevelGroups(t *testing.T) {
	org := getOrgWithGroups(t)
	req := NewRequestf(t, "GET", "/api/v1/orgs/org-with-groups/groups").AddBasicAuth("user4")
	resp := MakeRequest(t, req, http.StatusOK)
	groups := DecodeJSON(t, resp, []api.Group{})
	expectedLen := unittest.GetCount(t, new(group_model.Group),
		group_model.FindGroupsOptions{
			ParentGroupID: 0,
			OwnerID:       org.ID,
		}.ToConds())
	assert.NotEqual(t, expectedLen, len(groups))
}

func testGroupNotAccessibleWhenParentIsPrivate(t *testing.T) {
}
