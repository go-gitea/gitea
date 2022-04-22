// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIGetCommentAttachment(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := db.AssertExistsAndLoadBean(t, &models.Comment{ID: 2}).(*models.Comment)
	assert.NoError(t, comment.LoadIssue())
	assert.NoError(t, comment.LoadAttachments())
	attachment := db.AssertExistsAndLoadBean(t, &models.Attachment{ID: comment.Attachments[0].ID}).(*models.Attachment)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: comment.Issue.RepoID}).(*models.Repository)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d/assets/%d", repoOwner.Name, repo.Name, comment.ID, attachment.ID)
	resp := session.MakeRequest(t, req, http.StatusOK)
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d/assets/%d?token=%s", repoOwner.Name, repo.Name, comment.ID, attachment.ID, token)
	resp = session.MakeRequest(t, req, http.StatusOK)

	var apiAttachment api.Attachment
	DecodeJSON(t, resp, &apiAttachment)

	expect := convert.ToAttachment(attachment)
	assert.Equal(t, expect.ID, apiAttachment.ID)
	assert.Equal(t, expect.Name, apiAttachment.Name)
	assert.Equal(t, expect.UUID, apiAttachment.UUID)
	assert.Equal(t, expect.Created.Unix(), apiAttachment.Created.Unix())
}

func TestAPIListCommentAttachments(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := db.AssertExistsAndLoadBean(t, &models.Comment{ID: 2}).(*models.Comment)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d/assets",
		repoOwner.Name, repo.Name, comment.ID)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiAttachments []*api.Attachment
	DecodeJSON(t, resp, &apiAttachments)
	expectedCount := db.GetCount(t, &models.Attachment{CommentID: comment.ID})
	assert.EqualValues(t, expectedCount, len(apiAttachments))
	db.AssertExistsAndLoadBean(t, &models.Attachment{ID: apiAttachments[0].ID, CommentID: comment.ID})
}

func TestAPICreateCommentAttachment(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := db.AssertExistsAndLoadBean(t, &models.Comment{ID: 2}).(*models.Comment)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
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

	db.AssertExistsAndLoadBean(t, &models.Attachment{ID: apiAttachment.ID, CommentID: comment.ID})
}

func TestAPIEditCommentAttachment(t *testing.T) {
	defer prepareTestEnv(t)()
	const newAttachmentName = "newAttachmentName"

	attachment := db.AssertExistsAndLoadBean(t, &models.Attachment{ID: 6}).(*models.Attachment)
	comment := db.AssertExistsAndLoadBean(t, &models.Comment{ID: attachment.CommentID}).(*models.Comment)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d/assets/%d?token=%s",
		repoOwner.Name, repo.Name, comment.ID, attachment.ID, token)
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		"name": newAttachmentName,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	db.AssertExistsAndLoadBean(t, &models.Attachment{ID: apiAttachment.ID, CommentID: comment.ID, Name: apiAttachment.Name})
}

func TestAPIDeleteCommentAttachment(t *testing.T) {
	defer prepareTestEnv(t)()

	attachment := db.AssertExistsAndLoadBean(t, &models.Attachment{ID: 6}).(*models.Attachment)
	comment := db.AssertExistsAndLoadBean(t, &models.Comment{ID: attachment.CommentID}).(*models.Comment)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d/assets/%d?token=%s",
		repoOwner.Name, repo.Name, comment.ID, attachment.ID, token)

	req := NewRequestf(t, "DELETE", urlStr)
	session.MakeRequest(t, req, http.StatusNoContent)

	db.AssertNotExistsBean(t, &models.Attachment{ID: attachment.ID, CommentID: comment.ID})
}
