package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/auth"
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

func createFileInRepoInGroup(t *testing.T, token, orgName string, groupID int64, repoName string, expectedStatus int) {
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/group/%d/%s/contents/a-new-file.txt", orgName, groupID, repoName),
		&api.CreateFileOptions{
			ContentBase64: "QnJpYW4gVGF0bGVyIGZ1Y2tlZCBhbmQgYWJ1c2VkIFNlYW4gSGFycmlzCkJyaWFuIFRhdGxlciBmdWNrZWQgYW5kIGFidXNlZCBTZWFuIEhhcnJpcwpCcmlhbiBUYXRsZXIgZnVja2VkIGFuZCBhYnVzZWQgU2VhbiBIYXJyaXMKQnJpYW4gVGF0bGVyIGZ1Y2tlZCBhbmQgYWJ1c2VkIFNlYW4gSGFycmlzCkJyaWFuIFRhdGxlciBmdWNrZWQgYW5kIGFidXNlZCBTZWFuIEhhcnJpcwpCcmlhbiBUYXRsZXIgZnVja2VkIGFuZCBhYnVzZWQgU2VhbiBIYXJyaXMKQnJpYW4gVGF0bGVyIGZ1Y2tlZCBhbmQgYWJ1c2VkIFNlYW4gSGFycmlzCkJyaWFuIFRhdGxlciBmdWNrZWQgYW5kIGFidXNlZCBTZWFuIEhhcnJpcwpCcmlhbiBUYXRsZXIgZnVja2VkIGFuZCBhYnVzZWQgU2VhbiBIYXJyaXMKQnJpYW4gVGF0bGVyIGZ1Y2tlZCBhbmQgYWJ1c2VkIFNlYW4gSGFycmlzCg0K",
		}).AddTokenAuth(token)
	MakeRequest(t, req, expectedStatus)
}

func getIssuesInRepoInGroup(t *testing.T, token, orgName string, groupID int64, repoName string, expectedStatus int) {
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/group/%d/%s/issues", orgName, groupID, repoName).AddTokenAuth(token)
	MakeRequest(t, req, expectedStatus)
}

func createIssueInRepoInGroup(t *testing.T, token, orgName string, groupID int64, repoName string, opts *api.CreateIssueOption, expectedStatus int) *api.Issue {
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/group/%d/%s/issues", orgName, groupID, repoName), opts).AddTokenAuth(token)
	resp := MakeRequest(t, req, expectedStatus)
	if expectedStatus == http.StatusCreated {
		return DecodeJSON(t, resp, &api.Issue{})
	}
	return nil
}

func deleteIssueInRepoInGroup(t *testing.T, token, orgName string, groupID int64, repoName string, issueIndex int64, expectedStatus int) {
	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/group/%d/%s/issues/%d", orgName, groupID, repoName, issueIndex).AddTokenAuth(token)
	MakeRequest(t, req, expectedStatus)
}

func TestAPIGroupTeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("TeamAssignmentInheritance", testTeamAssignmentInheritance)
	t.Run("TeamAssignmentAndAccess", testRepoGroupTeamAssignmentAndAccess)
}

func testTeamAssignmentInheritance(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, url *url.URL) {
		data := createOrgWithGroups(t)
		const actor = "user2"
		adminToken := getUserToken(t, actor, auth.AccessTokenScopeWriteOrganization)
		tid := data.teamMembers[groupOrgUnitWriterTeam].tid
		unprivilegedActor := unittest.AssertExistsAndLoadBean(t, &user_model.User{
			ID: data.teamMembers[groupOrgUnitWriterTeam].uid,
		})
		unprivilegedToken := getUserToken(t, unprivilegedActor.Name, auth.AccessTokenScopeWriteOrganization, auth.AccessTokenScopeWriteRepository, auth.AccessTokenScopeWriteIssue)

		editReq := NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/teams/%d", tid),
			&api.EditTeamOption{
				UnitsMap: map[string]string{
					unit_model.Units[unit_model.TypeIssues].NameKey: "read",
					unit_model.Units[unit_model.TypeCode].NameKey:   "write",
				},
			}).AddTokenAuth(adminToken)
		MakeRequest(t, editReq, http.StatusOK)

		makeIssuesRequests := func(groupID int64, repoName string, expectedListStatus, expectedCreateStatus, expectedWriteStatus int) {
			getIssuesInRepoInGroup(t, unprivilegedToken, data.org.Name, groupID, repoName, expectedListStatus)
			toDelete := createIssueInRepoInGroup(t, unprivilegedToken, data.org.Name, groupID, repoName,
				&api.CreateIssueOption{
					Title: "an issue to be deleted",
					Body:  "...",
				},
				expectedCreateStatus)
			if toDelete != nil {
				deleteIssueInRepoInGroup(t, unprivilegedToken, data.org.Name, groupID, repoName, toDelete.Index, expectedWriteStatus)
			}
		}

		t.Run("RootGroupDefaults", func(t *testing.T) {
			rootGrp := createGroup(t, actor, data.org.Name, 0, &api.NewGroupOption{
				Name:        "root-level access test",
				Description: "should inherit permissions from org",
			}, http.StatusCreated)
			assertGroupTeamAndUnitsExist(t, rootGrp.ID, data.teamMembers[groupOrgUnitWriterTeam].tid, map[string]string{
				unit_model.Units[unit_model.TypeIssues].NameKey: "read",
				unit_model.Units[unit_model.TypeCode].NameKey:   "write",
			})

			createRepoInGroup(t, data.org.Name, actor, rootGrp.ID, "a-repo", http.StatusCreated)

			createFileInRepoInGroup(t, unprivilegedToken, data.org.Name, rootGrp.ID, "a-repo", http.StatusCreated)
			makeIssuesRequests(rootGrp.ID, "a-repo", http.StatusOK, http.StatusCreated, http.StatusForbidden)
		})
		t.Run("ParentChildOverrides", func(t *testing.T) {
			parentGrp := createGroup(t, actor, data.org.Name, 0, &api.NewGroupOption{
				Name: "parent test group",
			}, http.StatusCreated)
			umap := map[string]string{
				unit_model.Units[unit_model.TypeIssues].NameKey: "none",
				unit_model.Units[unit_model.TypeCode].NameKey:   "read",
			}
			editGroupTeam(t, actor, parentGrp.ID, groupOrgUnitWriterTeam, &api.CreateOrUpdateRepoGroupTeamOption{
				CanCreateIn: new(false),
				Permission:  new(api.RepoWritePermissionRead),
				UnitsMap:    umap,
			}, http.StatusNoContent)
			childGrp := createGroup(t, actor, data.org.Name, parentGrp.ID, &api.NewGroupOption{
				Name:       "child test group",
				Visibility: api.VisibleTypePrivate,
			}, http.StatusCreated)
			unittest.AssertCount(t, &group_model.RepoGroupTeam{
				TeamID:      tid,
				GroupID:     childGrp.ID,
				CanCreateIn: false,
			}, 1)
			assertGroupTeamAndUnitsExist(t, childGrp.ID, tid, umap)

			createRepoInGroup(t, data.org.Name, actor, childGrp.ID, "another-repo", http.StatusCreated)
			createFileInRepoInGroup(t, unprivilegedToken, data.org.Name, childGrp.ID, "another-repo", http.StatusForbidden)
			makeIssuesRequests(childGrp.ID, "another-repo", http.StatusNotFound, http.StatusNotFound, http.StatusNotFound)
		})
		t.Run("NearestAncestorOverrides", func(t *testing.T) {
			grandparent := createGroup(t, actor, data.org.Name, 0, &api.NewGroupOption{
				Name: "grandparent",
			}, http.StatusCreated)
			umap := map[string]string{
				unit_model.Units[unit_model.TypeIssues].NameKey: "none",
				unit_model.Units[unit_model.TypeCode].NameKey:   "read",
			}
			editGroupTeam(t, actor, grandparent.ID, groupOrgUnitWriterTeam, &api.CreateOrUpdateRepoGroupTeamOption{
				CanCreateIn: new(false),
				Permission:  new(api.RepoWritePermissionRead),
				UnitsMap:    umap,
			}, http.StatusNoContent)
			parent := createGroup(t, actor, data.org.Name, grandparent.ID, &api.NewGroupOption{
				Name: "parent",
			}, http.StatusCreated)
			child := createGroup(t, actor, data.org.Name, parent.ID, &api.NewGroupOption{
				Name:       "child",
				Visibility: api.VisibleTypePrivate,
			}, http.StatusCreated)

			assertGroupTeamAndUnitsExist(t, grandparent.ID, tid, umap)
			assertGroupTeamAndUnitsExist(t, child.ID, tid, umap)

			createRepoInGroup(t, data.org.Name, actor, child.ID, "another-repo", http.StatusCreated)
			createFileInRepoInGroup(t, unprivilegedToken, data.org.Name, child.ID, "another-repo", http.StatusForbidden)
			makeIssuesRequests(child.ID, "another-repo", http.StatusNotFound, http.StatusNotFound, http.StatusForbidden)
		})
	})
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

			editGroupTeam(t, actor, ngrp.ID, c.teamName, &api.CreateOrUpdateRepoGroupTeamOption{
				UnitsMap:    c.unitMap,
				CanCreateIn: new(c.expectedStatuses.createGroup == http.StatusCreated),
				Permission:  new(permToRepoWritePermission(data.teamMembers[c.teamName].perm)),
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
				editGroupTeam(t, actor, ngrp.ID, groupOrgUnitTeam, &api.CreateOrUpdateRepoGroupTeamOption{
					Permission: new(api.RepoWritePermissionWrite),
				}, http.StatusNoContent)
			}

			editGroupTeam(t, caseActor.Name, ngrp.ID, groupOrgUnitTeam, &api.CreateOrUpdateRepoGroupTeamOption{
				Permission: new(api.RepoWritePermissionRead),
			}, c.expectedStatuses.updateTeam)
		})
	}
}
