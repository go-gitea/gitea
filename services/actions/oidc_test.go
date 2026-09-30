// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobWorkflowFile_LocalCallsInheritCallerRefs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	chain := buildCallerChain(t, "./.gitea/workflows/a.yml", "user2/repo1/.gitea/workflows/b.yml@master", "./.gitea/workflows/c.yml")
	chain[1].ReusableWorkflowRef = "refs/heads/master"
	for _, job := range chain[1:] {
		job.WorkflowSourceRepoID, job.WorkflowSourceCommitSHA = 1, "lib-sha"
		_, err := actions_model.UpdateRunJob(t.Context(), job, nil, "workflow_source_repo_id", "workflow_source_commit_sha", "reusable_workflow_ref")
		require.NoError(t, err)
	}
	root := workflowFile{repo: "user2/repo1", ref: "refs/heads/main"}

	file, err := jobWorkflowFile(t.Context(), chain[1], root)
	require.NoError(t, err)
	assert.Equal(t, "user2/repo1/.gitea/workflows/a.yml@refs/heads/main", file.String())

	file, err = jobWorkflowFile(t.Context(), &actions_model.ActionRunJob{RunID: chain[2].RunID, ParentJobID: chain[2].ID, WorkflowSourceRepoID: 1, WorkflowSourceCommitSHA: "lib-sha"}, root)
	require.NoError(t, err)
	assert.Equal(t, workflowFile{repo: "user2/repo1", path: ".gitea/workflows/c.yml", ref: "refs/heads/master", sha: "lib-sha"}, file)
}
