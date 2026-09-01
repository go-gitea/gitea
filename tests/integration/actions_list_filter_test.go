// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsListFilters(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, user5.Name)
	actionsURL := fmt.Sprintf("/%s/%s/actions", user5.Name, repo.Name)

	t.Run("BranchDropdownListsBranches", func(t *testing.T) {
		req := NewRequest(t, "GET", actionsURL)
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		var labels []string
		htmlDoc.doc.Find(`[data-test-id="filter-branch"] .menu a.item`).Each(func(_ int, a *goquery.Selection) {
			labels = append(labels, strings.TrimSpace(a.Text()))
		})
		assert.Contains(t, labels, "master")
	})

	t.Run("FilterByBranch", func(t *testing.T) {
		req := NewRequest(t, "GET", actionsURL+"?branch=master")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		refs := htmlDoc.doc.Find(".run-list .run-list-ref")
		assert.Positive(t, refs.Length(), "filtered run list should not be empty")
		refs.Each(func(_ int, sel *goquery.Selection) {
			assert.Equal(t, "master", strings.TrimSpace(sel.Text()))
		})
	})

	t.Run("PaginationPreservesFilters", func(t *testing.T) {
		req := NewRequest(t, "GET", actionsURL+"?branch=master&limit=1")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		pageLinks := htmlDoc.doc.Find(".pagination a[href]")
		assert.Positive(t, pageLinks.Length(), "pagination should be rendered")
		pageLinks.Each(func(_ int, a *goquery.Selection) {
			u, err := url.Parse(a.AttrOr("href", ""))
			require.NoError(t, err)
			assert.Equal(t, "master", u.Query().Get("branch"), "pagination link must preserve branch filter")
		})
	})
}
