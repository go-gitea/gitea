// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/translation"

	"github.com/stretchr/testify/assert"
)

func TestViewBranches(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/branches")
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	_, exists := htmlDoc.doc.Find(".delete-branch-button").Attr("data-url")
	assert.False(t, exists, "The template has changed")
}

func TestDeleteBranch(t *testing.T) {
	defer prepareTestEnv(t)()

	deleteBranch(t)
}

func TestUndoDeleteBranch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		deleteBranch(t)
		htmlDoc, name := branchAction(t, ".undo-button")
		assert.Contains(t,
			htmlDoc.doc.Find(".ui.positive.message").Text(),
			translation.NewLocale("en-US").Tr("repo.branch.restore_success", name),
		)
	})
}

func deleteBranch(t *testing.T) {
	htmlDoc, name := branchAction(t, ".delete-branch-button")
	assert.Contains(t,
		htmlDoc.doc.Find(".ui.positive.message").Text(),
		translation.NewLocale("en-US").Tr("repo.branch.deletion_success", name),
	)
}

func branchAction(t *testing.T, button string) (*HTMLDoc, string) {
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/branches")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(button).Attr("data-url")
	if !assert.True(t, exists, "The template has changed") {
		t.Skip()
	}

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	session.MakeRequest(t, req, http.StatusOK)

	url, err := url.Parse(link)
	assert.NoError(t, err)
	req = NewRequest(t, "GET", "/user2/repo1/branches")
	resp = session.MakeRequest(t, req, http.StatusOK)

	return NewHTMLParser(t, resp.Body), url.Query().Get("name")
}
