// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueDependencyCandidates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	enableRepoDependencies(t, 1)

	// the open issues of repo1 are, newest first: pull 11 (index 5), pull 3, pull 2 and issue 1
	// issue 1 is blocked by pull 2 and blocks pull 3, pull 2 also blocks pull 11
	require.NoError(t, db.Insert(t.Context(), []*issues_model.IssueDependency{
		{UserID: 2, IssueID: 1, DependencyID: 2},
		{UserID: 2, IssueID: 3, DependencyID: 1},
		{UserID: 2, IssueID: 11, DependencyID: 2},
	}))

	candidates := func(t *testing.T, session *TestSession, link string) []int64 {
		resp := session.MakeRequest(t, NewRequest(t, "GET", link), http.StatusOK)
		ids := []int64{}
		for _, issue := range DecodeJSON(t, resp, []*api.Issue{}) {
			ids = append(ids, issue.ID)
		}
		return ids
	}
	user2 := loginUser(t, "user2")

	t.Run("SameRepository", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.AllowCrossRepositoryDependencies, false)()

		assert.Equal(t, []int64{11}, candidates(t, user2, "/user2/repo1/issues/1/dependency/candidates"))
		assert.Equal(t, []int64{3}, candidates(t, user2, "/user2/repo1/pulls/2/dependency/candidates"))
		assert.Equal(t, []int64{3, 1}, candidates(t, user2, "/user2/repo1/pulls/5/dependency/candidates"))
		// numeric keywords are searched in the database, so the test doesn't wait for the indexer
		assert.Equal(t, []int64{11}, candidates(t, user2, "/user2/repo1/issues/1/dependency/candidates?q=5"))
		assert.Equal(t, []int64{1}, candidates(t, user2, "/user2/repo1/pulls/5/dependency/candidates?q=1"))
		assert.Empty(t, candidates(t, user2, "/user2/repo1/issues/1/dependency/candidates?q=2"))
	})

	t.Run("FullPage", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.AllowCrossRepositoryDependencies, false)()

		// pull 11 and pull 2 come before pull 3 but are left out, so they must not take a place of the page
		assert.Equal(t, []int64{3}, candidates(t, user2, "/user2/repo1/pulls/2/dependency/candidates?limit=1"))
		assert.Equal(t, []int64{3}, candidates(t, user2, "/user2/repo1/pulls/5/dependency/candidates?limit=1"))
	})

	t.Run("CrossRepository", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.AllowCrossRepositoryDependencies, true)()

		ids := candidates(t, user2, "/user2/repo1/issues/1/dependency/candidates?limit=50")
		assert.Contains(t, ids, int64(11))
		assert.Contains(t, ids, int64(7)) // issue 7 is in user2's private repo2
		assert.NotContains(t, ids, int64(1))
		assert.NotContains(t, ids, int64(2))
		assert.NotContains(t, ids, int64(3))

		// user40 can write to repo1 but can't read repo2
		ids = candidates(t, loginUser(t, "user40"), "/user2/repo1/issues/1/dependency/candidates?limit=50")
		assert.Contains(t, ids, int64(11))
		assert.NotContains(t, ids, int64(7))
	})

	t.Run("Permission", func(t *testing.T) {
		// user4 can read repo1 but can't add dependencies to its issues
		loginUser(t, "user4").MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1/dependency/candidates"), http.StatusForbidden)
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1/dependency/candidates"), http.StatusSeeOther)
	})
}
