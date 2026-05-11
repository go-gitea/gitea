// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitStatusDescription(t *testing.T) {
	cases := []struct {
		status           actions_model.Status
		started, stopped timeutil.TimeStamp
		want             string
	}{
		{actions_model.StatusSuccess, 100, 102, "Successful in 2s"},
		{actions_model.StatusFailure, 100, 130, "Failing after 30s"},
		{actions_model.StatusCancelled, 100, 145, "Cancelled after 45s"},
		{actions_model.StatusSkipped, 0, 0, "Skipped"},
		{actions_model.StatusRunning, 0, 0, "In progress"},
		{actions_model.StatusWaiting, 0, 0, "Waiting to run"},
		{actions_model.StatusBlocked, 0, 0, "Blocked by required conditions"},
		{actions_model.StatusUnknown, 0, 0, "Unknown status: 0"},
	}
	for _, tc := range cases {
		job := &actions_model.ActionRunJob{Status: tc.status, Started: tc.started, Stopped: tc.stopped}
		assert.Equal(t, tc.want, toCommitStatusDescription(job), tc.status.String())
	}
}

func TestCreateCommitStatus_Dedupe(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	commit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
	require.NoError(t, err)

	run := &actions_model.ActionRun{
		ID:         99001,
		RepoID:     repo.ID,
		Repo:       repo,
		WorkflowID: "status-dedupe-test.yaml",
	}
	job := &actions_model.ActionRunJob{
		ID:     99002,
		RunID:  run.ID,
		RepoID: repo.ID,
		Name:   "status-dedupe-job",
		Status: actions_model.StatusWaiting,
	}

	expectedContext := "status-dedupe-test.yaml / status-dedupe-job (push)"
	expectedTargetURL := run.Link() + "/jobs/99002"

	require.NoError(t, createCommitStatus(t.Context(), repo, "push", commit.ID.String(), run, job))

	statuses := findCommitStatusesForContext(t, repo.ID, commit.ID.String(), expectedContext)
	require.Len(t, statuses, 1)
	assert.Equal(t, commitstatus.CommitStatusPending, statuses[0].State)
	assert.Equal(t, "Waiting to run", statuses[0].Description)
	assert.Equal(t, expectedTargetURL, statuses[0].TargetURL)

	job.Status = actions_model.StatusRunning
	require.NoError(t, createCommitStatus(t.Context(), repo, "push", commit.ID.String(), run, job))

	statuses = findCommitStatusesForContext(t, repo.ID, commit.ID.String(), expectedContext)
	require.Len(t, statuses, 2)
	assert.Equal(t, "Waiting to run", statuses[0].Description)
	assert.Equal(t, commitstatus.CommitStatusPending, statuses[1].State)
	assert.Equal(t, "In progress", statuses[1].Description)
	assert.Equal(t, expectedTargetURL, statuses[1].TargetURL)

	require.NoError(t, createCommitStatus(t.Context(), repo, "push", commit.ID.String(), run, job))
	statuses = findCommitStatusesForContext(t, repo.ID, commit.ID.String(), expectedContext)
	assert.Len(t, statuses, 2)

	job.Status = actions_model.StatusSuccess
	require.NoError(t, createCommitStatus(t.Context(), repo, "push", commit.ID.String(), run, job))
	statuses = findCommitStatusesForContext(t, repo.ID, commit.ID.String(), expectedContext)
	require.Len(t, statuses, 3)
	assert.Equal(t, commitstatus.CommitStatusSuccess, statuses[2].State)
}

func TestGetCommitActionsStatusMap(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo.ID, Name: repo.DefaultBranch})

	run := &actions_model.ActionRun{
		RepoID: repo.ID, Repo: repo, OwnerID: repo.OwnerID, TriggerUserID: repo.OwnerID,
		WorkflowID: "test.yaml", CommitSHA: branch.CommitID,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	cases := []struct {
		jobName string
		status  actions_model.Status
	}{
		{"running-job", actions_model.StatusRunning},
		{"waiting-job", actions_model.StatusWaiting},
		{"unknown-job", actions_model.StatusUnknown},
	}
	for _, tc := range cases {
		job := &actions_model.ActionRunJob{
			RunID: run.ID, RepoID: repo.ID, OwnerID: repo.OwnerID, Name: tc.jobName, Status: tc.status,
		}
		require.NoError(t, db.Insert(t.Context(), job))
		require.NoError(t, createCommitStatus(t.Context(), repo, "push", branch.CommitID, run, job))
	}

	statuses, err := git_model.GetLatestCommitStatus(t.Context(), repo.ID, branch.CommitID, db.ListOptionsAll)
	require.NoError(t, err)

	info := actions_module.GetCommitActionsStatusMap(t.Context(), statuses)
	got := map[string]string{}
	for _, s := range statuses {
		got[s.Context] = info.IconStatus(s)
	}
	for _, tc := range cases {
		key := "test.yaml / " + tc.jobName + " (push)"
		want := tc.status.String()
		assert.Equal(t, want, got[key], "icon status for %s", tc.jobName)
	}

	// Nil receiver returns "" without panicking — used by callers that skip enrichment.
	var nilInfo actions_module.CommitActionsStatusMap
	assert.Empty(t, nilInfo.IconStatus(statuses[0]))
}

func findCommitStatusesForContext(t *testing.T, repoID int64, sha, context string) []*git_model.CommitStatus {
	t.Helper()

	var statuses []*git_model.CommitStatus
	err := db.GetEngine(t.Context()).
		Where("repo_id = ? AND sha = ? AND context = ?", repoID, sha, context).
		Asc("`index`").
		Find(&statuses)
	require.NoError(t, err)
	return statuses
}
