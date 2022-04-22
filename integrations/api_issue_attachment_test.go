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
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIGetIssueAttachment(t *testing.T) {
	defer prepareTestEnv(t)()

	attachment := db.AssertExistsAndLoadBean(t, &models.Attachment{ID: 1}).(*models.Attachment)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: attachment.RepoID}).(*models.Repository)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{RepoID: attachment.IssueID}).(*models.Issue)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d?token=%s",
		repoOwner.Name, repo.Name, issue.Index, attachment.ID, token)

	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	db.AssertExistsAndLoadBean(t, &models.Attachment{ID: apiAttachment.ID, IssueID: issue.ID})
}

func TestAPIListIssueAttachments(t *testing.T) {
	defer prepareTestEnv(t)()

	attachment := db.AssertExistsAndLoadBean(t, &models.Attachment{ID: 1}).(*models.Attachment)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: attachment.RepoID}).(*models.Repository)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{RepoID: attachment.IssueID}).(*models.Issue)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets?token=%s",
		repoOwner.Name, repo.Name, issue.Index, token)

	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	apiAttachment := new([]api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	db.AssertExistsAndLoadBean(t, &models.Attachment{ID: (*apiAttachment)[0].ID, IssueID: issue.ID})
}

func TestAPICreateIssueAttachment(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{RepoID: repo.ID}).(*models.Issue)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets?token=%s",
		repoOwner.Name, repo.Name, issue.Index, token)

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

	db.AssertExistsAndLoadBean(t, &models.Attachment{ID: apiAttachment.ID, IssueID: issue.ID})
}

func TestAPIEditIssueAttachment(t *testing.T) {
	defer prepareTestEnv(t)()
	const newAttachmentName = "newAttachmentName"

	attachment := db.AssertExistsAndLoadBean(t, &models.Attachment{ID: 1}).(*models.Attachment)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: attachment.RepoID}).(*models.Repository)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{RepoID: attachment.IssueID}).(*models.Issue)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d?token=%s",
		repoOwner.Name, repo.Name, issue.Index, attachment.ID, token)
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		"name": newAttachmentName,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	apiAttachment := new(api.Attachment)
	DecodeJSON(t, resp, &apiAttachment)

	db.AssertExistsAndLoadBean(t, &models.Attachment{ID: apiAttachment.ID, IssueID: issue.ID, Name: apiAttachment.Name})
}

func TestAPIDeleteIssueAttachment(t *testing.T) {
	defer prepareTestEnv(t)()

	attachment := db.AssertExistsAndLoadBean(t, &models.Attachment{ID: 1}).(*models.Attachment)
	repo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: attachment.RepoID}).(*models.Repository)
	issue := db.AssertExistsAndLoadBean(t, &models.Issue{RepoID: attachment.IssueID}).(*models.Issue)
	repoOwner := db.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/assets/%d?token=%s",
		repoOwner.Name, repo.Name, issue.Index, attachment.ID, token)

	req := NewRequest(t, "DELETE", urlStr)
	session.MakeRequest(t, req, http.StatusNoContent)

	db.AssertNotExistsBean(t, &models.Attachment{ID: attachment.ID, IssueID: issue.ID})
}
