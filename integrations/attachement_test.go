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
	myImage := image.NewRGBA(image.Rect(0, 0, 32, 32))
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

	csrf := GetCSRF(t, session, repoURL)

	req := NewRequestWithBody(t, "POST", "/attachments", body)
	req.Header.Add("X-Csrf-Token", csrf)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	resp := session.MakeRequest(t, req, expectedStatus)

	if expectedStatus != http.StatusOK {
		return ""
	}
	var obj map[string]string
	DecodeJSON(t, resp, &obj)
	return obj["uuid"]
}

func TestCreateAnonymousAttachment(t *testing.T) {
	prepareTestEnv(t)
	session := emptyTestSession(t)
	createAttachment(t, session, "user2/repo1", "image.png", generateImg(), http.StatusFound)
}

func TestCreateIssueAttachement(t *testing.T) {
	prepareTestEnv(t)
	const repoURL = "user2/repo1"
	session := loginUser(t, "user2")
	uuid := createAttachment(t, session, repoURL, "image.png", generateImg(), http.StatusOK)

	req := NewRequest(t, "GET", repoURL+"/issues/new")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form").Attr("action")
	assert.True(t, exists, "The template has changed")

	postData := map[string]string{
		"_csrf":    htmlDoc.GetCSRF(),
		"title":    "New Issue With Attachement",
		"content":  "some content",
		"files[0]": uuid,
	}

	req = NewRequestWithValues(t, "POST", link, postData)
	resp = session.MakeRequest(t, req, http.StatusFound)
	test.RedirectURL(resp) // check that redirect URL exists

	//Validate that attachement is available
	req = NewRequest(t, "GET", "/attachments/"+uuid)
	session.MakeRequest(t, req, http.StatusOK)
}
