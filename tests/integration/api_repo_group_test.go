// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	group_model "code.gitea.io/gitea/models/group"
	perm_model "code.gitea.io/gitea/models/perm"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

const (
	groupOrgAdminTeam      = "group-admins"
	groupOrgWriterTeam     = "group-writers"
	groupOrgReaderTeam     = "group-readers"
	groupOrgUnitTeam       = "unit-specialists"
	groupOrgUnitWriterTeam = "unit-writer-specialists"
)

type commonGroupRepoTestData struct {
	fullFeatured          *api.Repository
	codeOnly              *api.Repository
	repoLevelTeamOverride *api.Repository
}
type commonGroupTestData struct {
	org                     *api.Organization
	rootPublic              *api.Group
	childPublic             *api.Group
	rootPrivate             *api.Group
	childPrivate            *api.Group
	privateGrandchildPublic *api.Group
	repos                   commonGroupRepoTestData
	teamMembers             map[string]*groupAccessAndUser
}

type groupAccessAndUser struct {
	uid  int64
	tid  int64
	perm perm_model.AccessMode
}

func createOrgWithGroups(t *testing.T) *commonGroupTestData {
	const actor = "user2"
	token := getUserToken(t, actor, auth_model.AccessTokenScopeWriteOrganization)
	const orgName = "org-with-groups"
	suffix := strconv.FormatInt(rand.Int64N(9999), 10)
	org := api.CreateOrgOption{
		UserName:    orgName + "-" + suffix,
		FullName:    "Org with groups #" + suffix,
		Description: "This organization has subgroups",
		Website:     "https://try.gitea.io",
		Location:    "Brian Tatler's walls",
		Visibility:  "public",
	}
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &org).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	apiOrg := DecodeJSON(t, resp, &api.Organization{})

	teamPrivs := map[string]*groupAccessAndUser{
		groupOrgAdminTeam:      {uid: 4, perm: perm_model.AccessModeAdmin},
		groupOrgWriterTeam:     {uid: 5, perm: perm_model.AccessModeWrite},
		groupOrgReaderTeam:     {uid: 8, perm: perm_model.AccessModeRead},
		groupOrgUnitTeam:       {uid: 13, perm: perm_model.AccessModeRead},
		groupOrgUnitWriterTeam: {uid: 14, perm: perm_model.AccessModeWrite},
	}

	baseOrgURL := "/api/v1/orgs/" + apiOrg.Name

	for k, v := range teamPrivs {
		reqBody := &api.CreateTeamOption{
			Name:                    k,
			CanCreateOrgRepo:        v.perm >= perm_model.AccessModeWrite,
			UnitsMap:                map[string]string{},
			IncludesAllRepositories: v.perm >= perm_model.AccessModeWrite,
			Permission:              api.RepoWritePermission(v.perm.ToString()),
		}
		for _, nunit := range unit_model.AllUnitKeyNames() {
			reqBody.UnitsMap[nunit] = v.perm.ToString()
		}
		treq := NewRequestWithJSON(t, "POST", baseOrgURL+"/teams", reqBody).AddTokenAuth(token)
		tres := MakeRequest(t, treq, http.StatusCreated)
		team := DecodeJSON(t, tres, &api.Team{})

		teamUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: v.uid})

		mreq := NewRequestf(t, "PUT", "/api/v1/teams/%d/members/%s", team.ID, teamUser.Name).AddTokenAuth(token)
		MakeRequest(t, mreq, http.StatusNoContent)
		teamPrivs[k].tid = team.ID
	}
	rootPublic := createGroup(t, actor, apiOrg.Name, 0, &api.NewGroupOption{
		Visibility: api.VisibleTypePublic,
		Name:       "root public",
	}, http.StatusCreated)
	childPublic := createGroup(t, actor, apiOrg.Name, rootPublic.ID, &api.NewGroupOption{
		Name:       "child public",
		Visibility: api.VisibleTypePublic,
	}, http.StatusCreated)
	rootPrivate := createGroup(t, actor, apiOrg.Name, 0, &api.NewGroupOption{
		Name:       "root private",
		Visibility: api.VisibleTypePrivate,
	}, http.StatusCreated)
	childPrivate := createGroup(t, actor, apiOrg.Name, rootPrivate.ID, &api.NewGroupOption{
		Name:       "child private",
		Visibility: api.VisibleTypePublic,
	}, http.StatusCreated)

	privateGrandchildPublic := createGroup(t, actor, apiOrg.Name, childPrivate.ID, &api.NewGroupOption{
		Name:       "public grandchild with private ancestors",
		Visibility: api.VisibleTypePublic,
	}, http.StatusCreated)

	val := &commonGroupTestData{
		org:                     apiOrg,
		rootPublic:              rootPublic,
		childPublic:             childPublic,
		rootPrivate:             rootPrivate,
		childPrivate:            childPrivate,
		privateGrandchildPublic: privateGrandchildPublic,
		repos: commonGroupRepoTestData{
			fullFeatured:          createRepoInGroup(t, apiOrg.Name, actor, childPrivate.ID, "full-featured-repo", http.StatusCreated),
			codeOnly:              createRepoInGroup(t, apiOrg.Name, actor, privateGrandchildPublic.ID, "code-only-repo", http.StatusCreated),
			repoLevelTeamOverride: createRepoInGroup(t, apiOrg.Name, actor, childPrivate.ID, "unit-repo", http.StatusCreated),
		},
		teamMembers: teamPrivs,
	}

	return val
}

func getGroup(t *testing.T, actor string, gid int64, expectedStatus int) *api.Group {
	token := getUserToken(t, actor, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestf(t, "GET", "/api/v1/groups/%d", gid).AddTokenAuth(token)
	res := MakeRequest(t, req, expectedStatus)
	if expectedStatus == 200 {
		return DecodeJSON(t, res, &api.Group{})
	}
	return nil
}

func getGroupSubgroups(t *testing.T, orgName, actor string, parentGroupID int64) []api.Group {
	token := getUserToken(t, actor, auth_model.AccessTokenScopeWriteOrganization)
	var endpoint string
	if parentGroupID <= 0 {
		endpoint = "/api/v1/orgs/" + orgName + "/groups"
	} else {
		endpoint = "/api/v1/groups/" + strconv.FormatInt(parentGroupID, 10) + "/subgroups"
	}
	req := NewRequest(t, "GET", endpoint).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	return DecodeJSON(t, resp, []api.Group{})
}

func createRepoInGroup(t *testing.T, orgName, actor string, parentGroupID int64, name string, expectedStatus int) *api.Repository {
	token := getUserToken(t, actor, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/"+orgName+"/repos", &api.CreateRepoOption{
		GroupID: parentGroupID,
		Name:    name,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedStatus)
	return DecodeJSON(t, resp, &api.Repository{})
}

func createGroup(t *testing.T, actor, orgName string, parentGroupID int64, options *api.NewGroupOption, expectedStatus int) *api.Group {
	token := getUserToken(t, actor, auth_model.AccessTokenScopeWriteOrganization)
	var endpoint string
	if parentGroupID <= 0 {
		endpoint = "/api/v1/orgs/" + orgName + "/groups/new"
	} else {
		endpoint = "/api/v1/groups/" + strconv.FormatInt(parentGroupID, 10) + "/new"
	}

	req := NewRequestWithJSON(t, "POST", endpoint, options).AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedStatus)
	if expectedStatus == http.StatusCreated {
		return DecodeJSON(t, resp, &api.Group{})
	}
	return nil
}

func editGroup(t *testing.T, actor string, groupID int64, options *api.EditGroupOption, expectedStatus int) *api.Group {
	token := getUserToken(t, actor, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/groups/"+strconv.FormatInt(groupID, 10), options).AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedStatus)
	if expectedStatus == http.StatusOK {
		return DecodeJSON(t, resp, &api.Group{})
	}
	return nil
}

func moveGroup(t *testing.T, actor string, groupID, newGroupID int64, pos *int, expectedStatus int) *api.Group {
	token := getUserToken(t, actor, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "POST", "/api/v1/groups/"+strconv.FormatInt(groupID, 10)+"/move", &api.MoveGroupOption{
		NewPos:    pos,
		NewParent: newGroupID,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedStatus)
	if expectedStatus == http.StatusOK {
		return DecodeJSON(t, resp, &api.Group{})
	}
	return nil
}

func assertGroupOrderSanity(t *testing.T, actor, orgName string, groupID int64, extraAssertion ...func(g *api.Group, idx int)) {
	subgroups := getGroupSubgroups(t, orgName, actor, groupID)
	for i, subgroup := range subgroups {
		assert.Equal(t, i, subgroup.SortOrder)
		if len(extraAssertion) > 0 {
			extraAssertion[0](&subgroup, i)
		}
	}
}

func TestAPIGroup(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("Creation", testCreateGroup)
	t.Run("Moving", testMoveGroup)
	t.Run("Visibility", testGroupVisibility)
}

func testCreateGroup(t *testing.T) {
	data := createOrgWithGroups(t)
	const actor = "user2"
	t.Run("RootLevelGroup", func(t *testing.T) {
		ng := createGroup(t, actor, data.org.Name, 0, &api.NewGroupOption{
			Name: "root level group x",
		}, http.StatusCreated)
		assert.Equal(t, int64(0), ng.ParentGroupID)
		assertGroupOrderSanity(t, actor, data.org.Name, 0)
	})
	t.Run("DepthOver20Fails", func(t *testing.T) {
		var gid int64
		for i := range int64(20) {
			ng := createGroup(t, actor, data.org.Name, gid, &api.NewGroupOption{
				Name:        "nested-group-" + strconv.FormatInt(i+1, 10),
				Description: "Group nested " + strconv.FormatInt(i+1, 10) + " levels deep",
			}, http.StatusCreated)
			gid = ng.ID
		}
		createGroup(t, actor, data.org.Name, gid, &api.NewGroupOption{
			Name:        "too deep",
			Description: "this should fail",
		}, http.StatusUnprocessableEntity)
	})
	t.Run("DenyCreateSubgroupToOutsider", func(t *testing.T) {
		createGroup(t, "user15", data.org.Name, data.rootPublic.ID, &api.NewGroupOption{
			Name:        "-",
			Description: "should fail",
		}, http.StatusNotFound)
	})
}

func testMoveGroup(t *testing.T) {
	data := createOrgWithGroups(t)
	const actor = "user2"
	t.Run("MoveGroupToOtherGroup", func(t *testing.T) {
		groupToMove := createGroup(t, actor, data.org.Name, 0, &api.NewGroupOption{
			Name: "movable group 1",
		}, http.StatusCreated)
		ng := moveGroup(t, actor, groupToMove.ID, data.rootPublic.ID, nil, http.StatusOK)
		assert.Equal(t, data.rootPublic.ID, ng.ParentGroupID)
		assert.Equal(t, 1, ng.SortOrder)
	})
	t.Run("MoveGroupToRoot", func(t *testing.T) {
		groupToMove := createGroup(t, actor, data.org.Name, data.rootPublic.ID, &api.NewGroupOption{
			Name: "movable group 2",
		}, http.StatusCreated)
		ng := moveGroup(t, actor, groupToMove.ID, 0, new(0), http.StatusOK)
		assert.Equal(t, int64(0), ng.ParentGroupID)
		assertGroupOrderSanity(t, actor, data.org.Name, 0, func(g *api.Group, idx int) {
			if idx == 0 {
				assert.Equal(t, ng.ID, g.ID)
			}
		})
	})
}

func testGroupVisibility(t *testing.T) {
	data := createOrgWithGroups(t)
	t.Run("OwnersAndSiteAdminsCanSeeAllTopLevelGroups", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/orgs/%s/groups", data.org.Name).AddBasicAuth("user2")
		resp := MakeRequest(t, req, http.StatusOK)
		groups := DecodeJSON(t, resp, []api.Group{})
		expectedLen := unittest.GetCount(t, new(group_model.Group),
			group_model.FindGroupsOptions{
				ParentGroupID: 0,
				OwnerID:       data.org.ID,
			}.ToConds())
		assert.Len(t, groups, expectedLen)

		// now test if site-wide admin can see all groups
		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/groups", data.org.Name).AddBasicAuth("user1")
		resp = MakeRequest(t, req, http.StatusOK)
		groups = DecodeJSON(t, resp, []api.Group{})
		assert.Len(t, groups, expectedLen)
	})
	t.Run("NonOrgMemberWontSeeHiddenTopLevelGroups", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/orgs/%s/groups", data.org.Name).AddBasicAuth("user12")
		resp := MakeRequest(t, req, http.StatusOK)
		groups := DecodeJSON(t, resp, []api.Group{})
		expectedLen := unittest.GetCount(t, new(group_model.Group),
			group_model.FindGroupsOptions{
				ParentGroupID: 0,
				OwnerID:       data.org.ID,
			}.ToConds())
		assert.NotEqual(t, expectedLen, len(groups))
	})
	t.Run("GroupsAndReposNotAccessibleWhenParentIsPrivate", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/groups/%d", data.privateGrandchildPublic.ID)
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequestf(t, "GET", "/api/v1/repos/%s", data.repos.fullFeatured.FullName)
		MakeRequest(t, req, http.StatusNotFound)
	})
	t.Run("ReposAndGroupsAccessibleToAdminsWhenParentIsPrivate", func(t *testing.T) {
		users := []string{"user1", "user2"}
		for _, u := range users {
			token := getUserToken(t, u, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeReadRepository)
			req := NewRequestf(t, "GET", "/api/v1/groups/%d", data.privateGrandchildPublic.ID).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusOK)
			req = NewRequestf(t, "GET", "/api/v1/repos/%s", data.repos.fullFeatured.FullName).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusOK)
		}
	})
	t.Run("PublicGroupIsAccessible", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/groups/%d", data.rootPublic.ID)
		MakeRequest(t, req, http.StatusOK)
	})
}

/*func testGroupNotAccessibleWhenParentIsPrivate(t *testing.T) {
}*/
