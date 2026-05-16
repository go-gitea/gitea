// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/dbfs"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/storage"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression for https://gitea.com/gitea/runner/issues/950: a runner that
// finalizes a task with no log output sends UpdateLog{Rows:[], NoMore:true}.
// The previous short-circuit on len(Rows)==0 skipped TransferLogs, leaving
// an orphan dbfs_data row. Verify the row is now archived and removed.
func TestActionsLogFinalizeWithoutRows(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-finalize-no-rows", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		const wfTreePath = ".gitea/workflows/finalize-no-rows.yml"
		wfFileContent := fmt.Sprintf(`name: finalize-no-rows
on:
  push:
    paths:
      - '%s'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: noop
`, wfTreePath)
		createWorkflowFile(t, token, user2.Name, repo.Name, wfTreePath, getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "trigger", wfFileContent))

		task := runner.fetchTask(t)

		resp, err := runner.client.runnerServiceClient.UpdateLog(t.Context(), connect.NewRequest(&runnerv1.UpdateLogRequest{
			TaskId: task.Id,
			Index:  0,
			Rows:   nil,
			NoMore: true,
		}))
		require.NoError(t, err)
		assert.EqualValues(t, 0, resp.Msg.AckIndex)

		freshTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.Id})
		require.True(t, freshTask.LogInStorage, "log_in_storage must flip after empty NoMore=true")

		_, err = storage.Actions.Stat(freshTask.LogFilename)
		assert.NoError(t, err, "archived log must exist in storage")

		_, err = dbfs.Open(t.Context(), actions_module.DBFSPrefix+freshTask.LogFilename)
		assert.ErrorIs(t, err, os.ErrNotExist, "DBFS row must be cleaned up after TransferLogs")
	})
}
