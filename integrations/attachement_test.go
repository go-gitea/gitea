// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/test"
	"github.com/stretchr/testify/assert"
)

func generateImg() bytes.Buffer {
	// Generate image
	myImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)
	return buff
}

func createAttachment(t *testing.T, session *TestSession, repoURL, filename string, buff bytes.Buffer, expectedStatus int) string {
	body := &bytes.Buffer{}

	//Setup multi-part
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	assert.NoError(t, err)
	_, err = io.Copy(part, &buff)
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST", repoURL+"/attachments", body)
	resp := session.MakeRequest(t, req, expectedStatus)

	var obj map[string]string
	DecodeJSON(t, resp, &obj)
	return obj["uuid"]
}

func TestCreateAnonymeAttachement(t *testing.T) {
	prepareTestEnv(t)
	createAttachment(t, nil, "user1/repo2", "image.png", generateImg(), http.StatusForbidden)
}

func TestCreateIssueAttachement(t *testing.T) {
	prepareTestEnv(t)
	const repoURL = "user1/repo2"
	session := loginUser(t, "user2")
	uuid := createAttachment(t, nil, repoURL, "image.png", generateImg(), http.StatusOK)

	req := NewRequest(t, "GET", repoURL+"/issues/new")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form").Attr("action")
	assert.True(t, exists, "The template has changed")

	postData := map[string]string{
		"_csrf":    htmlDoc.GetCSRF(),
		"title":    "New Issue With Attachement",
		"content":  "",
		"files[0]": uuid,
	}

	req = NewRequestWithValues(t, "POST", link, postData)
	resp = session.MakeRequest(t, req, http.StatusFound)
	test.RedirectURL(resp) // check that redirect URL exists

	//TODO validate
}

/*


func TestCreateDisaledAttachement(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	//setting.AttachmentEnabled
}

func TestCreateNotAllowedAttachement(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	//setting.AttachmentEnabled
}

func TestCreateIssueCommentAttachement(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	//TODO upload attachement
	//TODO create issue comment with attachement
	//TODO validate
}

func TestCreateReleaseAttachement(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	//TODO upload attachement
	//TODO create release with attachement
	createNewRelease(t, session, "/user2/repo1", "test-attachement", "test-attachement", false, true)
	//TODO validate
}

func TestCreateUnlinkedAttachement(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	//TODO upload attachement
	//TODO try to get attachement
}

func TestCreateUnauthorizedAttachement(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	//TODO upload attachement
	//TODO try to get attachement from an unauthorized user
}
*/
