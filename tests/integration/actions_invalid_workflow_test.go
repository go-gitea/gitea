// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/json"
	"gitea.dev/routers/web/repo/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsInvalidWorkflowPush(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		repo := createActionsTestRepo(t, token, "invalid-workflow-push", false)
		createWorkflowFile(t, token, user.Name, repo.Name, ".gitea/workflows/invalid.yml",
			getWorkflowCreateFileOptions(user, repo.DefaultBranch, "invalid workflow", "on: push\njobs: {check: {if: unknown.x}}\n"))

		runs, err := db.Find[actions_model.ActionRun](t.Context(), actions_model.FindRunOptions{RepoID: repo.ID})
		require.NoError(t, err)
		require.Len(t, runs, 1)
		run := runs[0]
		assert.Equal(t, actions_model.StatusFailure, run.Status)

		statuses, err := git_model.GetLatestCommitStatus(t.Context(), repo.ID, run.CommitSHA, db.ListOptionsAll)
		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, commitstatus.CommitStatusFailure, statuses[0].State)

		view := session.MakeRequest(t, NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d", user.Name, repo.Name, run.ID)), http.StatusOK)
		var viewResponse actions.ViewResponse
		require.NoError(t, json.Unmarshal(view.Body.Bytes(), &viewResponse))
		assert.Empty(t, viewResponse.State.Run.Jobs)
		require.Len(t, viewResponse.State.Run.JobSummaries, 1)
		assert.Equal(t, "invalid.yml", viewResponse.State.Run.JobSummaries[0].JobName)
		assert.Contains(t, string(viewResponse.State.Run.JobSummaries[0].SummaryHTML), "Invalid workflow file: .gitea/workflows/invalid.yml")
		assert.Contains(t, string(viewResponse.State.Run.JobSummaries[0].SummaryHTML), "Unrecognized named-value: &#39;unknown&#39;")

		session.MakeRequest(t, NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user.Name, repo.Name, run.ID)), http.StatusBadRequest)
	})
}
