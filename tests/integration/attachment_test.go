// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"image"
	"image/png"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	route_web "code.gitea.io/gitea/routers/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGeneratePngBytes() []byte {
	myImage := image.NewRGBA(image.Rect(0, 0, 32, 32))
	var buff bytes.Buffer
	_ = png.Encode(&buff, myImage)
	return buff.Bytes()
}

func testCreateIssueAttachment(t *testing.T, session *TestSession, repoURL, filename string, content []byte, expectedStatus int) string {
	body := &bytes.Buffer{}

	// Setup multi-part
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	assert.NoError(t, err)
	_, err = part.Write(content)
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST", repoURL+"/issues/attachments", body)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	resp := session.MakeRequest(t, req, expectedStatus)

	if expectedStatus != http.StatusOK {
		return ""
	}
	var obj map[string]string
	DecodeJSON(t, resp, &obj)
	return obj["uuid"]
}

func TestAttachments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("CreateAnonymousAttachment", testCreateAnonymousAttachment)
	t.Run("CreateUser2IssueAttachment", testCreateUser2IssueAttachment)
	t.Run("UploadAttachmentDeleteTemp", testUploadAttachmentDeleteTemp)
	t.Run("GetAttachment", testGetAttachment)
}

func testUploadAttachmentDeleteTemp(t *testing.T) {
	session := loginUser(t, "user2")
	countTmpFile := func() int {
		// TODO: GOLANG-HTTP-TMPDIR: Golang saves the uploaded file to os.TempDir() when it exceeds the max memory limit.
		files, err := fs.Glob(os.DirFS(os.TempDir()), "multipart-*") //nolint:usetesting // Golang's "http" package's behavior
		require.NoError(t, err)
		return len(files)
	}
	var tmpFileCountDuringUpload int
	defer test.MockVariableValue(&context.ParseMultipartFormMaxMemory, 1)()
	defer web.RouteMock(route_web.RouterMockPointBeforeWebRoutes, func(resp http.ResponseWriter, req *http.Request) {
		tmpFileCountDuringUpload = countTmpFile()
	})()
	_ = testCreateIssueAttachment(t, session, "user2/repo1", "image.png", testGeneratePngBytes(), http.StatusOK)
	assert.Equal(t, 1, tmpFileCountDuringUpload, "the temp file should exist when uploaded size exceeds the parse form's max memory")
	assert.Equal(t, 0, countTmpFile(), "the temp file should be deleted after upload")
}

func testCreateAnonymousAttachment(t *testing.T) {
	session := emptyTestSession(t)
	testCreateIssueAttachment(t, session, "user2/repo1", "image.png", testGeneratePngBytes(), http.StatusSeeOther)
}

func testCreateUser2IssueAttachment(t *testing.T) {
	const repoURL = "user2/repo1"
	session := loginUser(t, "user2")
	uuid := testCreateIssueAttachment(t, session, repoURL, "image.png", testGeneratePngBytes(), http.StatusOK)

	req := NewRequest(t, "GET", repoURL+"/issues/new")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form#new-issue").Attr("action")
	assert.True(t, exists, "The template has changed")

	postData := map[string]string{
		"title":   "New Issue With Attachment",
		"content": "some content",
		"files":   uuid,
	}

	req = NewRequestWithValues(t, "POST", link, postData)
	resp = session.MakeRequest(t, req, http.StatusOK)
	test.RedirectURL(resp) // check that redirect URL exists

	// Validate that attachment is available
	req = NewRequest(t, "GET", "/attachments/"+uuid)
	session.MakeRequest(t, req, http.StatusOK)

	// anonymous visit should be allowed because user2/repo1 is a public repository
	MakeRequest(t, req, http.StatusOK)
}

func testGetAttachment(t *testing.T) {
	adminSession := loginUser(t, "user1")
	user2Session := loginUser(t, "user2")
	user8Session := loginUser(t, "user8")
	emptySession := emptyTestSession(t)
	testCases := []struct {
		name       string
		uuid       string
		createFile bool
		session    *TestSession
		want       int
	}{
		{"LinkedIssueUUID", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", true, user2Session, http.StatusOK},
		{"LinkedCommentUUID", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", true, user2Session, http.StatusOK},
		{"linked_release_uuid", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a19", true, user2Session, http.StatusOK},
		{"NotExistingUUID", "b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18", false, user2Session, http.StatusNotFound},
		{"FileMissing", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18", false, user2Session, http.StatusInternalServerError},
		{"NotLinked", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a20", true, user2Session, http.StatusNotFound},
		{"NotLinkedAccessibleByUploader", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a20", true, user8Session, http.StatusOK},
		{"PublicByNonLogged", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", true, emptySession, http.StatusOK},
		{"PrivateByNonLogged", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12", true, emptySession, http.StatusNotFound},
		{"PrivateAccessibleByAdmin", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12", true, adminSession, http.StatusOK},
		{"PrivateAccessibleByUser", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12", true, user2Session, http.StatusOK},
		{"RepoNotAccessibleByUser", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12", true, user8Session, http.StatusNotFound},
		{"OrgNotAccessibleByUser", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a21", true, user8Session, http.StatusNotFound},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write empty file to be available for response
			if tc.createFile {
				_, err := storage.Attachments.Save(repo_model.AttachmentRelativePath(tc.uuid), strings.NewReader("hello world"), -1)
				assert.NoError(t, err)
			}
			// Actual test
			req := NewRequest(t, "GET", "/attachments/"+tc.uuid)
			tc.session.MakeRequest(t, req, tc.want)
		})
	}
}
