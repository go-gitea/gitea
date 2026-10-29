// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIModifyLabels(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	t.Run("AddIssueLabelsWithLabelNames", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 6, RepoID: repo.ID})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		repoLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 10, RepoID: repo.ID})
		orgLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 4, OrgID: owner.ID})

		user1Session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, user1Session, auth_model.AccessTokenScopeWriteIssue)

		// add the org label and the repo label to the issue
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels", owner.Name, repo.Name, issue.Index)
		req := NewRequestWithJSON(t, "POST", urlStr, &api.IssueLabelsOption{
			Labels: []any{repoLabel.Name, orgLabel.Name},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		apiLabels := DecodeJSON(t, resp, []*api.Label{})
		assert.Len(t, apiLabels, unittest.GetCount(t, &issues_model.IssueLabel{IssueID: issue.ID}))
		var apiLabelNames []string
		for _, label := range apiLabels {
			apiLabelNames = append(apiLabelNames, label.Name)
		}
		assert.ElementsMatch(t, apiLabelNames, []string{repoLabel.Name, orgLabel.Name})

		// delete labels
		req = NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})

	for _, testCase := range []struct {
		name       string
		repoID     int64
		labelCount int
	}{
		{"Repository", 2, 2},
		{"Organization", 3, 4},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: testCase.repoID})
			owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
			username := owner.Name
			scopes := []auth_model.AccessTokenScope{auth_model.AccessTokenScopeWriteIssue}
			urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels", owner.Name, repo.Name)
			labelFilter := &issues_model.Label{RepoID: repo.ID}
			if testCase.name == "Organization" {
				username = "user1"
				scopes = []auth_model.AccessTokenScope{auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization}
				urlStr = fmt.Sprintf("/api/v1/orgs/%s/labels", owner.Name)
				labelFilter = &issues_model.Label{OrgID: owner.ID}
			}
			session := loginUser(t, username)
			token := getTokenForLoggedInUser(t, session, scopes...)

			// CreateLabel
			req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
				Name:        "TestL 1",
				Color:       "abcdef",
				Description: "test label",
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusCreated)
			apiLabel := DecodeJSON(t, resp, &api.Label{})
			labelFilter.ID = apiLabel.ID
			dbLabel := unittest.AssertExistsAndLoadBean(t, labelFilter)
			assert.Equal(t, dbLabel.Name, apiLabel.Name)
			assert.Equal(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

			req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
				Name:        "TestL 2",
				Color:       "#123456",
				Description: "jet another test label",
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
				Name:  "WrongTestL",
				Color: "#12345g",
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusUnprocessableEntity)

			// ListLabels
			req = NewRequest(t, "GET", urlStr).
				AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			apiLabels := DecodeJSON(t, resp, []*api.Label{})
			assert.Len(t, apiLabels, testCase.labelCount)

			// GetLabel
			singleURLStr := fmt.Sprintf("%s/%d", urlStr, dbLabel.ID)
			req = NewRequest(t, "GET", singleURLStr).
				AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			apiLabel = DecodeJSON(t, resp, &api.Label{})
			assert.Equal(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

			// EditLabel
			newName := "LabelNewName"
			newColor := "09876a"
			newColorWrong := "09g76a"
			req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
				Name:  &newName,
				Color: &newColor,
			}).AddTokenAuth(token)
			resp = MakeRequest(t, req, http.StatusOK)
			apiLabel = DecodeJSON(t, resp, &api.Label{})
			assert.Equal(t, newColor, apiLabel.Color)
			req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
				Color: &newColorWrong,
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusUnprocessableEntity)

			// DeleteLabel
			req = NewRequest(t, "DELETE", singleURLStr).
				AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNoContent)
		})
	}
}

func TestAPIAddIssueLabels(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	_ = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{RepoID: repo.ID, ID: 2})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels",
		repo.OwnerName, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.IssueLabelsOption{
		Labels: []any{1, 2},
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	apiLabels := DecodeJSON(t, resp, []*api.Label{})
	assert.Len(t, apiLabels, unittest.GetCount(t, &issues_model.IssueLabel{IssueID: issue.ID}))

	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: 2})
}

// TestAPIDeleteIssueLabelCrossRepo ensures DeleteIssueLabel does not act on a label
// belonging to another repository, and that a foreign-but-existing label ID and a
// nonexistent label ID return the same status (no cross-repo enumeration oracle).
func TestAPIDeleteIssueLabelCrossRepo(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	// label 5 exists but belongs to repo 10, not repo 1
	foreignLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 5})
	assert.NotEqual(t, repo.ID, foreignLabel.RepoID)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	base := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels", repo.OwnerName, repo.Name, issue.Index)

	// a foreign-but-existing label ID must not be accepted (was 204 before the fix)
	req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", base, foreignLabel.ID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// a nonexistent label ID must return the SAME status, so no oracle exists (was 422 before the fix)
	req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", base, 9999999)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// a label that belongs to the repo is still removable
	unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 2, RepoID: repo.ID})
	addReq := NewRequestWithJSON(t, "POST", base, &api.IssueLabelsOption{Labels: []any{2}}).AddTokenAuth(token)
	MakeRequest(t, addReq, http.StatusOK)
	req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%d", base, 2)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIAddIssueLabelsWithSharedRepoAndOrgLabelName(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 6, RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	repoLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 10, RepoID: repo.ID})
	orgOnlyLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 4, OrgID: owner.ID})

	assert.NoError(t, issues_model.NewLabel(t.Context(), &issues_model.Label{OrgID: owner.ID, Name: repoLabel.Name, Color: "#abcdef"}))

	token := getTokenForLoggedInUser(t, loginUser(t, "user1"), auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels", owner.Name, repo.Name, issue.Index)

	for _, method := range []string{"POST", "PUT"} {
		t.Run(method, func(t *testing.T) {
			req := NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequestWithJSON(t, method, urlStr, &api.IssueLabelsOption{
				Labels: []any{repoLabel.Name, orgOnlyLabel.Name},
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			apiLabels := DecodeJSON(t, resp, []*api.Label{})

			var apiLabelIDs []int64
			for _, label := range apiLabels {
				apiLabelIDs = append(apiLabelIDs, label.ID)
			}
			assert.ElementsMatch(t, []int64{repoLabel.ID, orgOnlyLabel.ID}, apiLabelIDs)
		})
	}
}

func TestAPIReplaceIssueLabels(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels",
		owner.Name, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "PUT", urlStr, &api.IssueLabelsOption{
		Labels: []any{label.ID},
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	apiLabels := DecodeJSON(t, resp, []*api.Label{})
	if assert.Len(t, apiLabels, 1) {
		assert.Equal(t, label.ID, apiLabels[0].ID)
	}

	unittest.AssertCount(t, &issues_model.IssueLabel{IssueID: issue.ID}, 1)
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
}

func TestAPIReplaceIssueLabelsWithLabelNames(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels",
		owner.Name, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "PUT", urlStr, &api.IssueLabelsOption{
		Labels: []any{label.Name},
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	apiLabels := DecodeJSON(t, resp, []*api.Label{})
	if assert.Len(t, apiLabels, 1) {
		assert.Equal(t, label.Name, apiLabels[0].Name)
	}
}
