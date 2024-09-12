// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIGetIssueAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: attachment.RepoID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: attachment.IssueID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d", repoOwner.Name, repo.Name, issue.Index, attachment.ID)).
		AddTokenAuth(token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, IssueID: issue.ID})
}

func TestAPIListIssueAttachments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: attachment.RepoID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: attachment.IssueID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets", repoOwner.Name, repo.Name, issue.Index)).
		AddTokenAuth(token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	apiAttachment := new([]api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: (*apiAttachment)[0].ID, IssueID: issue.ID})
}

func TestAPICreateIssueAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	filename := "image.png"
	buff := generateImg()
	body := &bytes.Buffer{}

	// Setup multi-part
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("attachment", filename)
	assert.NoError(t, err)
	_, err = io.Copy(part, &buff)
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets", repoOwner.Name, repo.Name, issue.Index), body).
		AddTokenAuth(token)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	resp := session.MakeRequest(t, req, http.StatusCreated)

	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, IssueID: issue.ID})
}

func TestAPICreateIssueAttachmentWithUnallowedFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	filename := "file.bad"
	body := &bytes.Buffer{}

	// Setup multi-part.
	writer := multipart.NewWriter(body)
	_, err := writer.CreateFormFile("attachment", filename)
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets", repoOwner.Name, repo.Name, issue.Index), body).
		AddTokenAuth(token)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	session.MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPIEditIssueAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const newAttachmentName = "newAttachmentName"

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: attachment.RepoID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: attachment.IssueID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d",
		repoOwner.Name, repo.Name, issue.Index, attachment.ID)
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		"name": newAttachmentName,
	}).AddTokenAuth(token)
	resp := session.MakeRequest(t, req, http.StatusCreated)
	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, IssueID: issue.ID, Name: apiAttachment.Name})
}

func TestAPIDeleteIssueAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: attachment.RepoID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: attachment.IssueID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d", repoOwner.Name, repo.Name, issue.Index, attachment.ID)).
		AddTokenAuth(token)
	session.MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &repo_model.Attachment{ID: attachment.ID, IssueID: issue.ID})
}
