// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/translation"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	req := NewRequest(t, "GET", "/user2/repo1/branches")
	resp := MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	AssertHTMLElement(t, htmlDoc, "[data-testid=branches-default-branch-list]", 1)
	AssertHTMLElement(t, htmlDoc, "[data-testid=branches-default-branch-not-exist]", 0)

	repo1.DefaultBranch = "non-existent-branch"
	_, _ = db.GetEngine(t.Context()).ID(repo1.ID).Cols("default_branch").Update(repo1)
	req = NewRequest(t, "GET", "/user2/repo1/branches")
	resp = MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	AssertHTMLElement(t, htmlDoc, "[data-testid=branches-default-branch-list]", 0)
	AssertHTMLElement(t, htmlDoc, "[data-testid=branches-default-branch-not-exist]", 1)
}

func TestUndoDeleteBranch(t *testing.T) {
	branchAction := func(t *testing.T, button, attr string) (*HTMLDoc, string) {
		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user2/repo1/branches")
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		link, exists := htmlDoc.doc.Find(button).Attr(attr)
		require.True(t, exists, "The template has changed")
		linkURL, err := url.Parse(link)
		require.NoError(t, err)

		req = NewRequest(t, "POST", link)
		session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", "/user2/repo1/branches")
		resp = session.MakeRequest(t, req, http.StatusOK)

		return NewHTMLParser(t, resp.Body), linkURL.Query().Get("name")
	}

	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		htmlDoc, name := branchAction(t, ".delete-branch-button", "data-modal-form.action")
		assert.Contains(t,
			htmlDoc.doc.Find(".ui.positive.message").Text(),
			translation.NewLocale("en-US").TrString("repo.branch.deletion_success", name),
		)
		htmlDoc, name = branchAction(t, ".restore-branch-button", "data-url")
		assert.Contains(t,
			htmlDoc.doc.Find(".ui.positive.message").Text(),
			translation.NewLocale("en-US").TrString("repo.branch.restore_success", name),
		)
	})
}
