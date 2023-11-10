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
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels?token=%s", owner.Name, repo.Name, token)

	// CreateLabel
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 1",
		Color:       "abcdef",
		Description: "test label",
	})
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
	})
	MakeRequest(t, req, http.StatusCreated)
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:  "WrongTestL",
		Color: "#12345g",
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// ListLabels
	req = NewRequest(t, "GET", urlStr)
	resp = MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, 2)

	// GetLabel
	singleURLStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels/%d?token=%s", owner.Name, repo.Name, dbLabel.ID, token)
	req = NewRequest(t, "GET", singleURLStr)
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
	})
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, newColor, apiLabel.Color)
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Color: &newColorWrong,
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// DeleteLabel
	req = NewRequest(t, "DELETE", singleURLStr)
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
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels?token=%s",
		repo.OwnerName, repo.Name, issue.Index, token)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.IssueLabelsOption{
		Labels: []int64{1, 2},
	})
	resp := MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, unittest.GetCount(t, &issues_model.IssueLabel{IssueID: issue.ID}))

	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: 2})
}

func TestAPIReplaceIssueLabels(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels?token=%s",
		owner.Name, repo.Name, issue.Index, token)
	req := NewRequestWithJSON(t, "PUT", urlStr, &api.IssueLabelsOption{
		Labels: []int64{label.ID},
	})
	resp := MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	if assert.Len(t, apiLabels, 1) {
		assert.EqualValues(t, label.ID, apiLabels[0].ID)
	}

	unittest.AssertCount(t, &issues_model.IssueLabel{IssueID: issue.ID}, 1)
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
}

func TestAPIModifyOrgLabels(t *testing.T) {
	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	user := "user1"
	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	urlStr := fmt.Sprintf("/api/v1/orgs/%s/labels?token=%s", owner.Name, token)

	// CreateLabel
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 1",
		Color:       "abcdef",
		Description: "test label",
	})
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
	})
	MakeRequest(t, req, http.StatusCreated)
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:  "WrongTestL",
		Color: "#12345g",
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// ListLabels
	req = NewRequest(t, "GET", urlStr)
	resp = MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, 4)

	// GetLabel
	singleURLStr := fmt.Sprintf("/api/v1/orgs/%s/labels/%d?token=%s", owner.Name, dbLabel.ID, token)
	req = NewRequest(t, "GET", singleURLStr)
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
	})
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, newColor, apiLabel.Color)
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Color: &newColorWrong,
	})
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// DeleteLabel
	req = NewRequest(t, "DELETE", singleURLStr)
	MakeRequest(t, req, http.StatusNoContent)
}
