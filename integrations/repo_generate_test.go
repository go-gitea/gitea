// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func testRepoGenerate(t *testing.T, session *TestSession, templateOwnerName, templateRepoName, generateOwnerName, generateRepoName string) *httptest.ResponseRecorder {
	generateOwner := models.AssertExistsAndLoadBean(t, &models.User{Name: generateOwnerName}).(*models.User)

	// Step0: check the existence of the generated repo
	req := NewRequestf(t, "GET", "/%s/%s", generateOwnerName, generateRepoName)
	resp := session.MakeRequest(t, req, http.StatusNotFound)

	// Step1: go to the main page of template repo
	req = NewRequestf(t, "GET", "/%s/%s", templateOwnerName, templateRepoName)
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Step2: click the "Use this template" button
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("a.ui.button[href^=\"/repo/create\"]").Attr("href")
	assert.True(t, exists, "The template has changed")
	req = NewRequest(t, "GET", link)
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Step3: fill the form of the create
	htmlDoc = NewHTMLParser(t, resp.Body)
	link, exists = htmlDoc.doc.Find("form.ui.form[action^=\"/repo/create\"]").Attr("action")
	assert.True(t, exists, "The template has changed")
	_, exists = htmlDoc.doc.Find(fmt.Sprintf(".owner.dropdown .item[data-value=\"%d\"]", generateOwner.ID)).Attr("data-value")
	assert.True(t, exists, fmt.Sprintf("Generate owner '%s' is not present in select box", generateOwnerName))
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":       htmlDoc.GetCSRF(),
		"uid":         fmt.Sprintf("%d", generateOwner.ID),
		"repo_name":   generateRepoName,
		"git_content": "true",
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	// Step4: check the existence of the generated repo
	req = NewRequestf(t, "GET", "/%s/%s", generateOwnerName, generateRepoName)
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestRepoGenerate(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user1")
	testRepoGenerate(t, session, "user27", "template1", "user1", "generated1")
}

func TestRepoGenerateToOrg(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")
	testRepoGenerate(t, session, "user27", "template1", "user2", "generated2")
}
