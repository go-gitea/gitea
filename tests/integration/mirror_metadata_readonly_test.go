// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

func TestReadOnlyMirrorRejectsIssueWrites(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	_, err := db.GetEngine(t.Context()).ID(repo.ID).Cols("is_mirror").Update(&repo_model.Repository{IsMirror: true})
	require.NoError(t, err)
	require.NoError(t, db.Insert(t.Context(), &repo_model.Mirror{RepoID: repo.ID, SyncIssues: true}))

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)
	apiURL := func(format string, args ...any) string {
		return fmt.Sprintf("/api/v1/repos/%s/%s", owner.Name, repo.Name) + fmt.Sprintf(format, args...)
	}

	t.Run("API", func(t *testing.T) {
		for _, req := range []*RequestWrapper{
			NewRequestWithJSON(t, "POST", apiURL("/issues"), &api.CreateIssueOption{Title: "local"}),
			NewRequestWithJSON(t, "POST", apiURL("/issues/1/comments"), &api.CreateIssueCommentOption{Body: "local"}),
			NewRequestWithJSON(t, "PATCH", apiURL("/issues/1"), &api.EditIssueOption{Title: "local"}),
			NewRequestWithJSON(t, "POST", apiURL("/issues/1/reactions"), &api.EditReactionOption{Reaction: "+1"}),
			NewRequestWithJSON(t, "POST", apiURL("/labels"), &api.CreateLabelOption{Name: "local", Color: "#000000"}),
			NewRequestWithJSON(t, "POST", apiURL("/milestones"), &api.CreateMilestoneOption{Title: "local"}),
		} {
			MakeRequest(t, req.AddTokenAuth(token), http.StatusLocked)
		}

		MakeRequest(t, NewRequest(t, "GET", apiURL("/issues/1")).AddTokenAuth(token), http.StatusOK)
		// watching a mirrored issue only changes the doer's own state
		MakeRequest(t, NewRequest(t, "PUT", apiURL("/issues/1/subscriptions/%s", owner.Name)).AddTokenAuth(token), http.StatusCreated)
	})

	t.Run("Web", func(t *testing.T) {
		session := loginUser(t, owner.Name)
		issueURL := fmt.Sprintf("/%s/%s/issues/1", owner.Name, repo.Name)
		session.MakeRequest(t, NewRequestWithValues(t, "POST", issueURL+"/comments", map[string]string{"content": "local"}), http.StatusNotFound)
		session.MakeRequest(t, NewRequestWithValues(t, "POST", issueURL+"/title", map[string]string{"title": "local"}), http.StatusNotFound)
		session.MakeRequest(t, NewRequest(t, "GET", issueURL), http.StatusOK)
		session.MakeRequest(t, NewRequestWithValues(t, "POST", issueURL+"/watch", map[string]string{"watch": "1"}), http.StatusOK)
	})
}
