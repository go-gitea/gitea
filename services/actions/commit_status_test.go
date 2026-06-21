// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/timeutil"

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
		{actions_model.StatusCancelled, 100, 145, "Canceled after 45s"},
		{actions_model.StatusCancelling, 0, 0, "Canceling"},
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

// TestCreateCommitStatus_DistinctWorkflowFilesSameName covers issue #35699:
// two workflow files with the same `name:` and same job name must produce
// two distinct commit statuses, not be deduplicated into one.
func TestCreateCommitStatus_DistinctWorkflowFilesSameName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo.ID, Name: repo.DefaultBranch})

	payload := []byte(`
name: test-run
on: pull_request
jobs:
  my-test:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`)

	for _, spec := range []struct {
		workflowID   string
		runID, jobID int64
	}{
		{"workflow1.yaml", 99101, 99201},
		{"workflow2.yaml", 99102, 99202},
	} {
		run := &actions_model.ActionRun{
			ID: spec.runID, Index: spec.runID, RepoID: repo.ID, Repo: repo, OwnerID: repo.OwnerID, TriggerUserID: repo.OwnerID,
			WorkflowID: spec.workflowID, CommitSHA: branch.CommitID,
		}
		require.NoError(t, db.Insert(t.Context(), run))
		job := &actions_model.ActionRunJob{
			ID: spec.jobID, RunID: run.ID, RepoID: repo.ID, OwnerID: repo.OwnerID,
			Name: "my-test", Status: actions_model.StatusWaiting,
			WorkflowPayload: payload,
		}
		require.NoError(t, db.Insert(t.Context(), job))
		require.NoError(t, createCommitStatus(t.Context(), repo, "pull_request", branch.CommitID, run, job))
	}

	statuses, err := git_model.GetLatestCommitStatus(t.Context(), repo.ID, branch.CommitID, db.ListOptionsAll)
	require.NoError(t, err)

	// Both workflow files should produce a row even though the display
	// Context is identical — matching GitHub's behavior.
	hashes := map[string]struct{}{}
	targets := map[string]struct{}{}
	for _, st := range statuses {
		hashes[st.ContextHash] = struct{}{}
		targets[st.TargetURL] = struct{}{}
		assert.Equal(t, "test-run / my-test (pull_request)", st.Context)
	}
	assert.Len(t, hashes, 2, "expected distinct ContextHash per workflow file")
	assert.Len(t, targets, 2, "expected distinct TargetURL per workflow file")
}

// TestCreateCommitStatus_LegacyHashRecovery covers the upgrade path: a pending
// status created before the fix (hashed from Context alone) must still be
// superseded by a follow-up event, instead of being orphaned in its own dedupe
// group while a new row accumulates under the new hash.
func TestCreateCommitStatus_LegacyHashRecovery(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo.ID, Name: repo.DefaultBranch})

	ctxName := "legacy.yaml / my-job (push)"
	legacyHash := git_model.HashCommitStatusContext(ctxName)
	sha, err := git.NewIDFromString(branch.CommitID)
	require.NoError(t, err)
	creator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	require.NoError(t, git_model.NewCommitStatus(t.Context(), git_model.NewCommitStatusOptions{
		Repo:    repo,
		Creator: creator,
		SHA:     sha,
		CommitStatus: &git_model.CommitStatus{
			State:       commitstatus.CommitStatusPending,
			Context:     ctxName,
			ContextHash: legacyHash,
			TargetURL:   "https://example.invalid/legacy",
			Description: "Waiting to run",
		},
	}))

	run := &actions_model.ActionRun{
		ID: 99301, Index: 99301, RepoID: repo.ID, Repo: repo, OwnerID: repo.OwnerID, TriggerUserID: repo.OwnerID,
		WorkflowID: "legacy.yaml", CommitSHA: branch.CommitID,
	}
	require.NoError(t, db.Insert(t.Context(), run))
	job := &actions_model.ActionRunJob{
		ID: 99302, RunID: run.ID, RepoID: repo.ID, OwnerID: repo.OwnerID,
		Name: "my-job", Status: actions_model.StatusSuccess,
	}
	require.NoError(t, db.Insert(t.Context(), job))
	require.NoError(t, createCommitStatus(t.Context(), repo, "push", branch.CommitID, run, job))

	latest, err := git_model.GetLatestCommitStatus(t.Context(), repo.ID, branch.CommitID, db.ListOptionsAll)
	require.NoError(t, err)
	// The new row must reuse the legacy hash so GetLatestCommitStatus returns
	// only one entry for this Context — the success, not the orphaned pending.
	matches := 0
	for _, s := range latest {
		if s.Context == ctxName {
			matches++
			assert.Equal(t, legacyHash, s.ContextHash)
			assert.Equal(t, commitstatus.CommitStatusSuccess, s.State)
		}
	}
	assert.Equal(t, 1, matches)
}

// TestCreateCommitStatus_UnnamedWorkflowUsesFileName: a workflow with no
// non-blank `name:` uses the file name in the Context, not an empty
// "/ job (event)" — covers both an omitted and a whitespace-only name.
func TestCreateCommitStatus_UnnamedWorkflowUsesFileName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: repo.ID, Name: repo.DefaultBranch})

	for _, tc := range []struct {
		workflowID   string
		runID, jobID int64
		payload      string
	}{
		{"unnamed.yaml", 99401, 99411, "on: push\n"},
		{"blank.yaml", 99402, 99412, "name: \"   \"\non: push\n"},
	} {
		run := &actions_model.ActionRun{
			ID: tc.runID, Index: tc.runID, RepoID: repo.ID, Repo: repo, OwnerID: repo.OwnerID, TriggerUserID: repo.OwnerID,
			WorkflowID: tc.workflowID, CommitSHA: branch.CommitID,
		}
		require.NoError(t, db.Insert(t.Context(), run))
		job := &actions_model.ActionRunJob{
			ID: tc.jobID, RunID: run.ID, RepoID: repo.ID, OwnerID: repo.OwnerID,
			Name: "my-test", Status: actions_model.StatusWaiting,
			WorkflowPayload: []byte(tc.payload + `jobs:
  my-test:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`),
		}
		require.NoError(t, db.Insert(t.Context(), job))
		require.NoError(t, createCommitStatus(t.Context(), repo, "push", branch.CommitID, run, job))

		statuses := findCommitStatusesForContext(t, repo.ID, branch.CommitID, tc.workflowID+" / my-test (push)")
		require.Len(t, statuses, 1)
		assert.Equal(t, commitstatus.CommitStatusPending, statuses[0].State)
	}
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
