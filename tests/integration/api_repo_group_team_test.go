package integration

import (
	"net/http"
	"testing"

	group_model "code.gitea.io/gitea/models/group"
	org_model "code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func createUnitMapWith(basePerm string, excludedVal string, excludeUnits ...unit_model.Unit) map[string]string {
	ret := make(map[string]string)
	excluded := make(map[unit_model.Type]bool)
	for _, v := range excludeUnits {
		excluded[v.Type] = true
	}
	for _, v := range unit_model.Units {
		if _, ok := excluded[v.Type]; !ok {
			ret[v.NameKey] = basePerm
		} else {
			ret[v.NameKey] = excludedVal
		}
	}
	return ret
}

func permToRepoWritePermission(p perm_model.AccessMode) api.RepoWritePermission {
	switch p {
	case perm_model.AccessModeAdmin, perm_model.AccessModeOwner:
		return api.RepoWritePermissionAdmin
	case perm_model.AccessModeWrite:
		return api.RepoWritePermissionWrite
	default:
		return api.RepoWritePermissionRead
	}
}

func assertGroupTeamAndUnitsExist(t *testing.T, groupID, teamID int64, unitMap map[string]string) {
	unittest.AssertCount(t, &group_model.RepoGroupTeam{
		TeamID:  teamID,
		GroupID: groupID,
	}, 1)
	for unitTypeString, permString := range unitMap {
		unitType := unit_model.TypeFromKey(unitTypeString)
		perm := perm_model.ParseAccessMode(permString)
		unittest.AssertCount(t, &group_model.RepoGroupUnit{
			Type:       unitType,
			GroupID:    groupID,
			TeamID:     teamID,
			AccessMode: perm,
		}, 1)
	}
}

func TestAPIGroupTeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("TeamAssignmentAndAccess", testRepoGroupTeamAssignmentAndAccess)
}

func testRepoGroupTeamAssignmentAndAccess(t *testing.T) {
	data := createOrgWithGroups(t)
	const actor = "user2"
	ngrp := createGroup(t, actor, data.org.Name, 0,
		&api.NewGroupOption{
			Name: "group for testing",
		},
		http.StatusCreated)
	type expectedStatusType struct {
		createGroup int
		editGroup   int
		updateTeam  int
	}
	excluded := []unit_model.Unit{
		unit_model.Units[unit_model.TypeExternalWiki],
		unit_model.Units[unit_model.TypeExternalTracker],
	}
	cases := []struct {
		unitMap          map[string]string
		teamName         string
		expectedStatuses expectedStatusType
	}{
		{
			teamName: groupOrgAdminTeam,
			unitMap:  createUnitMapWith("admin", "read", excluded...),
			expectedStatuses: expectedStatusType{
				createGroup: http.StatusCreated,
				editGroup:   http.StatusOK,
				updateTeam:  http.StatusNoContent,
			},
		},
		{
			teamName: groupOrgWriterTeam,
			unitMap:  createUnitMapWith("write", "read", excluded...),
			expectedStatuses: expectedStatusType{
				createGroup: http.StatusCreated,
				editGroup:   http.StatusOK,
				updateTeam:  http.StatusForbidden,
			},
		},
		{
			teamName: groupOrgReaderTeam,
			unitMap:  createUnitMapWith("read", "read"),
			expectedStatuses: expectedStatusType{
				createGroup: http.StatusForbidden,
				editGroup:   http.StatusForbidden,
				updateTeam:  http.StatusForbidden,
			},
		},
	}
	for i, c := range cases {
		t.Run("AssignmentTo_`"+c.teamName+"`", func(t *testing.T) {
			caseActor := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: data.teamMembers[c.teamName].uid})

			addGroupTeam(t, actor, ngrp.ID, c.teamName, &api.CreateOrUpdateRepoGroupTeamOption{
				UnitsMap:   c.unitMap,
				Permission: new(permToRepoWritePermission(data.teamMembers[c.teamName].perm)),
			}, http.StatusNoContent)
			team, err := org_model.GetTeam(t.Context(), data.org.ID, c.teamName)
			assert.NoError(t, err)
			assertGroupTeamAndUnitsExist(t, ngrp.ID, team.ID, c.unitMap)

			createGroup(t, caseActor.Name, data.org.Name, ngrp.ID, &api.NewGroupOption{
				Name:       c.teamName + " subgroup",
				Visibility: api.VisibleTypeLimited,
			}, c.expectedStatuses.createGroup)

			editGroup(t, caseActor.Name, ngrp.ID, &api.EditGroupOption{
				Description: new("new description"),
			}, c.expectedStatuses.editGroup)

			if i == 0 {
				addGroupTeam(t, actor, ngrp.ID, groupOrgUnitTeam, &api.CreateOrUpdateRepoGroupTeamOption{
					Permission: new(api.RepoWritePermissionWrite),
				}, http.StatusNoContent)
			}

			editGroupTeam(t, caseActor.Name, ngrp.ID, groupOrgUnitTeam, &api.CreateOrUpdateRepoGroupTeamOption{
				Permission: new(api.RepoWritePermissionRead),
			}, c.expectedStatuses.updateTeam)
		})
	}
}
