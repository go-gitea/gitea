// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	organization_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func getOrCreateOrgWithGroups(t *testing.T) *user_model.User {
	e := db.GetEngine(t.Context())
	norg := &user_model.User{
		LowerName: "org-with-groups",
		FullName:  "Org With Groups",
		Name:      "Org-With-Groups",
		Type:      user_model.UserTypeOrganization,
	}

	hasOrgWithGroups, err := e.Exist(&user_model.User{
		LowerName: norg.LowerName,
		Type:      norg.Type,
	})
	assert.NoError(t, err)
	if !hasOrgWithGroups {
		ownerBean := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			ID: 2,
		})
		assert.NoError(t, organization_model.CreateOrganization(t.Context(), organization_model.OrgFromUser(norg), ownerBean))
		_, err = e.Table(&group_model.Group{}).Update(&group_model.Group{
			OwnerName: norg.Name,
			OwnerID:   norg.ID,
		})
		assert.NoError(t, err)
	}
	norg = unittest.AssertExistsAndLoadBean(t, norg)
	return norg
}

func TestNewGroup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	orgWithGroups := getOrCreateOrgWithGroups(t)

	const groupName = "group x"
	group := &group_model.Group{
		Name:    groupName,
		OwnerID: orgWithGroups.ID,
	}
	assert.NoError(t, NewGroup(t.Context(), group))
	unittest.AssertExistsAndLoadBean(t, &group_model.Group{Name: groupName})
}

func TestMoveGroup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	orgWithGroups := getOrCreateOrgWithGroups(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		ID: 2,
	})
	testfn := func(gid int64) {
		cond := &group_model.FindGroupsOptions{
			ParentGroupID: 9,
			OwnerID:       orgWithGroups.ID,
		}
		origCount := unittest.GetCount(t, new(group_model.Group), cond.ToConds())

		assert.NoError(t, MoveGroupItem(t.Context(), MoveGroupOptions{
			NewParent: 9,
			ItemID:    gid,
			IsGroup:   true,
			NewPos:    -1,
		}, doer))
		unittest.AssertCountByCond(t, "repo_group", cond.ToConds(), origCount+1)
	}
	testfn(23)
	testfn(22)
	testfn(4)
}

func TestMoveRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		ID: 2,
	})
	orgWithGroups := getOrCreateOrgWithGroups(t)
	repoToMove, err := repo_service.CreateRepository(t.Context(), doer, orgWithGroups, repo_service.CreateRepoOptions{
		GroupID: 2,
		Name:    "Repo-to-move",
	})
	assert.NoError(t, err)
	cond := repo_model.SearchRepositoryCondition(repo_model.SearchRepoOptions{
		GroupID: 1,
	})
	origCount := unittest.GetCount(t, new(repo_model.Repository), cond)

	assert.NoError(t, MoveGroupItem(t.Context(), MoveGroupOptions{
		NewParent: 1,
		ItemID:    repoToMove.ID,
		IsGroup:   false,
		NewPos:    -1,
	}, doer))
	unittest.AssertCountByCond(t, "repository", cond, origCount+1)
}
