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
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.Equal(t, "Has started running", statuses[1].Description)
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
