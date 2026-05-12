// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRunJobsByRunAndAttemptIDOrder(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		repo := createActionsTestRepo(t, token, "actions-job-sort", false)

		run := &actions_model.ActionRun{
			Title:         "job sort test",
			RepoID:        repo.ID,
			OwnerID:       user2.ID,
			WorkflowID:    "job-sort.yml",
			Index:         1,
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/master",
			CommitSHA:     "deadbeef",
			Event:         "workflow_dispatch",
			TriggerEvent:  "workflow_dispatch",
			EventPayload:  "{}",
			Status:        actions_model.StatusWaiting,
		}
		require.NoError(t, db.Insert(t.Context(), run))

		mkJob := func(jobID, name string) *actions_model.ActionRunJob {
			return &actions_model.ActionRunJob{
				RunID:     run.ID,
				RepoID:    repo.ID,
				OwnerID:   user2.ID,
				CommitSHA: run.CommitSHA,
				Name:      name,
				Attempt:   1,
				JobID:     jobID,
				Status:    actions_model.StatusWaiting,
			}
		}

		// Insert matrix "test" jobs out of natural order so the assertion can detect the sort.
		jobs := []*actions_model.ActionRunJob{
			mkJob("build", "build"),
			mkJob("test", "test (10)"),
			mkJob("test", "test (2)"),
			mkJob("test", "test (1)"),
			mkJob("deploy", "deploy"),
		}
		for _, j := range jobs {
			require.NoError(t, db.Insert(t.Context(), j))
		}

		got, err := actions_model.GetRunJobsByRunAndAttemptID(t.Context(), run.ID, 0)
		require.NoError(t, err)
		require.Len(t, got, 5)

		gotNames := make([]string, len(got))
		for i, j := range got {
			gotNames[i] = j.Name
		}
		assert.Equal(t, []string{"build", "test (1)", "test (2)", "test (10)", "deploy"}, gotNames)
	})
}
