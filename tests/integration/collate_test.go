// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestMySQLCollate(t *testing.T) {
	// This test is only for MySQL, return early for any other engine.
	if !setting.Database.Type.IsMySQL() {
		t.Skip()
	}

	defer tests.PrepareTestEnv(t)()

	// Helpers
	loadProps := func() (*repo_model.Repository, *user_model.User, string) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

		return repo, owner, token
	}

	breakCollation := func() {
		err := db.ConvertCharsetAndCollation("utf8mb4", "utf8mb4_general_ci")
		assert.NoError(t, err)
	}
	fixCollation := func() {
		charset, collation, err := db.GetDesiredCharsetAndCollation()
		assert.NoError(t, err)
		err = db.ConvertCharsetAndCollation(charset, collation)
		assert.NoError(t, err)
	}

	t.Run("Collation fixing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Ensure that the database uses the wrong collation
		breakCollation()

		// With the wrong collation, sanity checking fails
		err := db.SanityCheck()
		assert.Error(t, err)

		// Try updating the collation
		fixCollation()

		// Sanity checking works after the collation update
		err = db.SanityCheck()
		assert.NoError(t, err)
	})

	t.Run("Case sensitive issue search by label", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		assert.NoError(t, unittest.LoadFixtures())

		// Helpers
		createLabel := func(name string) int64 {
			repo, owner, token := loadProps()
			urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels", owner.Name, repo.Name)

			// CreateLabel
			req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
				Name:        name,
				Color:       "abcdef",
				Description: "test label",
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusCreated)
			apiLabel := new(api.Label)
			DecodeJSON(t, resp, &apiLabel)
			return apiLabel.ID
		}

		createIssue := func(title string, labelID int64) {
			repo, owner, token := loadProps()
			urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues", owner.Name, repo.Name)

			// CreateIssue
			req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
				Title:  title,
				Labels: []int64{labelID},
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		searchIssues := func(label string) []*api.Issue {
			_, _, token := loadProps()
			var apiIssues []*api.Issue

			urlStr := fmt.Sprintf("/api/v1/repos/issues/search?labels=%s", label)
			req := NewRequest(t, "GET", urlStr).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)

			DecodeJSON(t, resp, &apiIssues)
			return apiIssues
		}

		// Ensure that the database uses the wrong collation
		breakCollation()

		// Create two labels that differ in case only
		labelID1 := createLabel("case-sens")
		labelID2 := createLabel("Case-Sens")

		// Create two issues, one with each of the labels above
		createIssue("case-sens 1", labelID1)
		createIssue("case-sens 2", labelID2)

		// Search for 'label1', and expect two results (`label1` and `Label1`)
		issues := searchIssues("case-sens")
		assert.Len(t, issues, 2)

		// Update the collation
		fixCollation()

		// Search for 'label1', and expect only one result now.
		issues = searchIssues("case-sens")
		assert.Len(t, issues, 1)
	})
}
