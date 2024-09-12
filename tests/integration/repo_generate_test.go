// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testRepoGenerate(t *testing.T, session *TestSession, templateID, templateOwnerName, templateRepoName, generateOwnerName, generateRepoName string) *httptest.ResponseRecorder {
	generateOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: generateOwnerName})

	// Step0: check the existence of the generated repo
	req := NewRequestf(t, "GET", "/%s/%s", generateOwnerName, generateRepoName)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Step1: go to the main page of template repo
	req = NewRequestf(t, "GET", "/%s/%s", templateOwnerName, templateRepoName)
	resp := session.MakeRequest(t, req, http.StatusOK)

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
		"_csrf":         htmlDoc.GetCSRF(),
		"uid":           fmt.Sprintf("%d", generateOwner.ID),
		"repo_name":     generateRepoName,
		"repo_template": templateID,
		"git_content":   "true",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Step4: check the existence of the generated repo
	req = NewRequestf(t, "GET", "/%s/%s", generateOwnerName, generateRepoName)
	session.MakeRequest(t, req, http.StatusOK)

	// Step5: check substituted values in Readme
	req = NewRequestf(t, "GET", "/%s/%s/raw/branch/master/README.md", generateOwnerName, generateRepoName)
	resp = session.MakeRequest(t, req, http.StatusOK)
	body := fmt.Sprintf(`# %s Readme
Owner: %s
Link: /%s/%s
Clone URL: %s%s/%s.git`,
		generateRepoName,
		strings.ToUpper(generateOwnerName),
		generateOwnerName,
		generateRepoName,
		setting.AppURL,
		generateOwnerName,
		generateRepoName)
	assert.Equal(t, body, resp.Body.String())

	// Step6: check substituted values in substituted file path ${REPO_NAME}
	req = NewRequestf(t, "GET", "/%s/%s/raw/branch/master/%s.log", generateOwnerName, generateRepoName, generateRepoName)
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, generateRepoName, resp.Body.String())

	return resp
}

func TestRepoGenerate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")
	testRepoGenerate(t, session, "44", "user27", "template1", "user1", "generated1")
}

func TestRepoGenerateToOrg(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	testRepoGenerate(t, session, "44", "user27", "template1", "user2", "generated2")
}
