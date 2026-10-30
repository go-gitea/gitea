// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssueAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: attachment.RepoID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: attachment.IssueID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)
	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	for _, operation := range []string{"get", "list", "edit-unallowed", "edit"} {
		t.Run(operation, func(t *testing.T) {
			token := readToken
			if operation == "edit-unallowed" || operation == "edit" {
				token = writeToken
			}

			switch operation {
			case "get":
				req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d", repoOwner.Name, repo.Name, issue.Index, attachment.ID)).
					AddTokenAuth(token)
				resp := session.MakeRequest(t, req, http.StatusOK)
				apiAttachment := DecodeJSON(t, resp, &api.Attachment{})

				unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, IssueID: issue.ID})

				commentAttachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 3, RepoID: repo.ID})
				req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d", repoOwner.Name, repo.Name, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: commentAttachment.IssueID}).Index, commentAttachment.ID)).
					AddTokenAuth(token)
				session.MakeRequest(t, req, http.StatusNotFound)
			case "list":
				req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets", repoOwner.Name, repo.Name, issue.Index)).
					AddTokenAuth(token)
				resp := session.MakeRequest(t, req, http.StatusOK)
				apiAttachment := DecodeJSON(t, resp, []api.Attachment{})

				unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment[0].ID, IssueID: issue.ID})
			case "edit-unallowed":
				issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: attachment.IssueID})
				filename := "file.bad"
				urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d", repoOwner.Name, repo.Name, issue.Index, attachment.ID)
				req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
					"name": filename,
				}).AddTokenAuth(token)

				session.MakeRequest(t, req, http.StatusUnprocessableEntity)
			case "edit":
				const newAttachmentName = "hello_world.txt"

				urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d",
					repoOwner.Name, repo.Name, issue.Index, attachment.ID)
				req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
					"name": newAttachmentName,
				}).AddTokenAuth(token)
				resp := session.MakeRequest(t, req, http.StatusCreated)
				apiAttachment := DecodeJSON(t, resp, &api.Attachment{})

				unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, IssueID: issue.ID, Name: apiAttachment.Name})
			}
		})
	}
}

func TestAPICreateIssueAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	for _, testCase := range []struct {
		filename string
		status   int
	}{
		{"file.bad", http.StatusUnprocessableEntity},
		{"image.png", http.StatusCreated},
	} {
		t.Run(testCase.filename, func(t *testing.T) {
			body := &bytes.Buffer{}

			// Setup multi-part
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("attachment", testCase.filename)
			assert.NoError(t, err)
			if testCase.status == http.StatusCreated {
				_, err = part.Write(testGeneratePngBytes())
				assert.NoError(t, err)
			}
			err = writer.Close()
			assert.NoError(t, err)

			req := NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets", repoOwner.Name, repo.Name, issue.Index), body).
				AddTokenAuth(token)
			req.Header.Add("Content-Type", writer.FormDataContentType())
			resp := session.MakeRequest(t, req, testCase.status)

			if testCase.status == http.StatusCreated {
				apiAttachment := DecodeJSON(t, resp, &api.Attachment{})
				unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, IssueID: issue.ID})
			}
		})
	}
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
