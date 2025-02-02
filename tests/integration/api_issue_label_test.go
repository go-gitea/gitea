// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIModifyLabels(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels", owner.Name, repo.Name)

	// CreateLabel
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 1",
		Color:       "abcdef",
		Description: "test label",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	apiLabel := new(api.Label)
	DecodeJSON(t, resp, &apiLabel)
	dbLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: apiLabel.ID, RepoID: repo.ID})
	assert.EqualValues(t, dbLabel.Name, apiLabel.Name)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

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
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, 2)

	// GetLabel
	singleURLStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels/%d", owner.Name, repo.Name, dbLabel.ID)
	req = NewRequest(t, "GET", singleURLStr).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

	// EditLabel
	newName := "LabelNewName"
	newColor := "09876a"
	newColorWrong := "09g76a"
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Name:  &newName,
		Color: &newColor,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, newColor, apiLabel.Color)
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Color: &newColorWrong,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// DeleteLabel
	req = NewRequest(t, "DELETE", singleURLStr).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
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
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, unittest.GetCount(t, &issues_model.IssueLabel{IssueID: issue.ID}))

	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: 2})
}

func TestAPIAddIssueLabelsWithLabelNames(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

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
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, unittest.GetCount(t, &issues_model.IssueLabel{IssueID: issue.ID}))
	var apiLabelNames []string
	for _, label := range apiLabels {
		apiLabelNames = append(apiLabelNames, label.Name)
	}
	assert.ElementsMatch(t, apiLabelNames, []string{repoLabel.Name, orgLabel.Name})

	// delete labels
	req = NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
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
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	if assert.Len(t, apiLabels, 1) {
		assert.EqualValues(t, label.ID, apiLabels[0].ID)
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
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	if assert.Len(t, apiLabels, 1) {
		assert.EqualValues(t, label.Name, apiLabels[0].Name)
	}
}

func TestAPIModifyOrgLabels(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	user := "user1"
	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	urlStr := fmt.Sprintf("/api/v1/orgs/%s/labels", owner.Name)

	// CreateLabel
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 1",
		Color:       "abcdef",
		Description: "test label",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	apiLabel := new(api.Label)
	DecodeJSON(t, resp, &apiLabel)
	dbLabel := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: apiLabel.ID, OrgID: owner.ID})
	assert.EqualValues(t, dbLabel.Name, apiLabel.Name)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

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
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, 4)

	// GetLabel
	singleURLStr := fmt.Sprintf("/api/v1/orgs/%s/labels/%d", owner.Name, dbLabel.ID)
	req = NewRequest(t, "GET", singleURLStr).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

	// EditLabel
	newName := "LabelNewName"
	newColor := "09876a"
	newColorWrong := "09g76a"
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Name:  &newName,
		Color: &newColor,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, newColor, apiLabel.Color)
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Color: &newColorWrong,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// DeleteLabel
	req = NewRequest(t, "DELETE", singleURLStr).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}
