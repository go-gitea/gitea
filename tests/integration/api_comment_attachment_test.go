// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIGetCommentAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2})
	assert.NoError(t, comment.LoadIssue(db.DefaultContext))
	assert.NoError(t, comment.LoadAttachments(db.DefaultContext))
	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: comment.Attachments[0].ID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: comment.Issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d/assets/%d", repoOwner.Name, repo.Name, comment.ID, attachment.ID)
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d/assets/%d?token=%s", repoOwner.Name, repo.Name, comment.ID, attachment.ID, token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiAttachment api.Attachment
	DecodeJSON(t, resp, &apiAttachment)

	expect := convert.ToAttachment(attachment)
	assert.Equal(t, expect.ID, apiAttachment.ID)
	assert.Equal(t, expect.Name, apiAttachment.Name)
	assert.Equal(t, expect.UUID, apiAttachment.UUID)
	assert.Equal(t, expect.Created.Unix(), apiAttachment.Created.Unix())
}

func TestAPIListCommentAttachments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d/assets",
		repoOwner.Name, repo.Name, comment.ID)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiAttachments []*api.Attachment
	DecodeJSON(t, resp, &apiAttachments)
	expectedCount := unittest.GetCount(t, &repo_model.Attachment{CommentID: comment.ID})
	assert.Len(t, apiAttachments, expectedCount)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachments[0].ID, CommentID: comment.ID})
}

func TestAPICreateCommentAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d/assets?token=%s",
		repoOwner.Name, repo.Name, comment.ID, token)

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

	req := NewRequestWithBody(t, "POST", urlStr, body)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	resp := session.MakeRequest(t, req, http.StatusCreated)

	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, CommentID: comment.ID})
}

func TestAPIEditCommentAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const newAttachmentName = "newAttachmentName"

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 6})
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: attachment.CommentID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d/assets/%d?token=%s",
		repoOwner.Name, repo.Name, comment.ID, attachment.ID, token)
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		"name": newAttachmentName,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: apiAttachment.ID, CommentID: comment.ID, Name: apiAttachment.Name})
}

func TestAPIDeleteCommentAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 6})
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: attachment.CommentID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: comment.IssueID})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d/assets/%d?token=%s",
		repoOwner.Name, repo.Name, comment.ID, attachment.ID, token)

	req := NewRequestf(t, "DELETE", urlStr)
	session.MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &repo_model.Attachment{ID: attachment.ID, CommentID: comment.ID})
}
