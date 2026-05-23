// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"fmt"
	"strings"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	db "code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildWorkflowTestRepo creates a temporary git repository for testing GetActionWorkflow.
// The default branch "main" has no workflow files; "feature" and "release-v1" each add their own workflow file.
func buildWorkflowTestRepo(t *testing.T) string {
	t.Helper()
	ctx := t.Context()
	tmpDir := t.TempDir()

	_, _, err := gitcmd.NewCommand("init").WithDir(tmpDir).RunStdString(ctx)
	require.NoError(t, err)

	readme := "readme"
	featureWF := "on: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo test\n"
	releaseWF := "on: [push]\njobs:\n  release:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo release\n"

	// Build a git fast-import stream:
	//   :4 = initial commit on main (README.md only)
	//   :5 = feature branch commit (adds feature workflow)
	//   :6 = release commit from :4 (adds release workflow, tagged release-v1, not on main)
	var sb strings.Builder
	fmt.Fprintf(&sb, "blob\nmark :1\ndata %d\n%s\n", len(readme), readme)
	fmt.Fprintf(&sb, "blob\nmark :2\ndata %d\n%s\n", len(featureWF), featureWF)
	fmt.Fprintf(&sb, "blob\nmark :3\ndata %d\n%s\n", len(releaseWF), releaseWF)
	fmt.Fprintf(&sb, "commit refs/heads/main\nmark :4\nauthor Test <test@gitea.com> 1000000000 +0000\ncommitter Test <test@gitea.com> 1000000000 +0000\ndata 14\ninitial commit\nM 100644 :1 README.md\n\n")
	fmt.Fprintf(&sb, "commit refs/heads/feature\nmark :5\nauthor Test <test@gitea.com> 1000000001 +0000\ncommitter Test <test@gitea.com> 1000000001 +0000\ndata 12\nadd workflow\nfrom :4\nM 100644 :2 .gitea/workflows/my-workflow.yml\n\n")
	fmt.Fprintf(&sb, "reset refs/pull/42/merge\nfrom :5\n\n")
	fmt.Fprintf(&sb, "commit refs/heads/main\nmark :6\nauthor Test <test@gitea.com> 1000000002 +0000\ncommitter Test <test@gitea.com> 1000000002 +0000\ndata 16\nrelease workflow\nfrom :4\nM 100644 :3 .gitea/workflows/my-workflow.yml\n\n")
	fmt.Fprintf(&sb, "reset refs/tags/release-v1\nfrom :6\n\n")
	fmt.Fprintf(&sb, "reset refs/heads/main\nfrom :4\n\n")
	fmt.Fprintf(&sb, "done\n")

	_, _, err = gitcmd.NewCommand("fast-import").WithDir(tmpDir).WithStdinBytes([]byte(sb.String())).RunStdString(ctx)
	require.NoError(t, err)

	return tmpDir
}

func TestGetActionWorkflow_FallbackRef(t *testing.T) {
	ctx := t.Context()

	repoDir := buildWorkflowTestRepo(t)

	gitRepo, err := git.OpenRepository(ctx, repoDir)
	require.NoError(t, err)
	defer gitRepo.Close()

	repo := &repo_model.Repository{
		DefaultBranch: "main",
		OwnerName:     "test-owner",
		Name:          "test-repo",
		Units: []*repo_model.RepoUnit{
			{
				Type:   unit.TypeActions,
				Config: &repo_model.ActionsConfig{},
			},
		},
	}

	t.Run("returns error when workflow only on non-default branch", func(t *testing.T) {
		_, err := GetActionWorkflow(ctx, gitRepo, repo, "my-workflow.yml")
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})

	t.Run("returns workflow when found via ref", func(t *testing.T) {
		wf, err := GetActionWorkflowByRef(ctx, gitRepo, repo, "my-workflow.yml", git.RefName("refs/heads/feature"))
		require.NoError(t, err)
		assert.Equal(t, "my-workflow.yml", wf.ID)
	})

	t.Run("returns workflow when found via pull ref", func(t *testing.T) {
		wf, err := GetActionWorkflowByRef(ctx, gitRepo, repo, "my-workflow.yml", git.RefName("refs/pull/42/merge"))
		require.NoError(t, err)
		assert.Equal(t, "my-workflow.yml", wf.ID)
		assert.Contains(t, wf.HTMLURL, "/src/commit/")
	})

	t.Run("returns workflow with tag link when found via tag ref", func(t *testing.T) {
		wf, err := GetActionWorkflowByRef(ctx, gitRepo, repo, "my-workflow.yml", git.RefName("refs/tags/release-v1"))
		require.NoError(t, err)
		assert.Equal(t, "my-workflow.yml", wf.ID)
		assert.Contains(t, wf.HTMLURL, "/src/tag/release-v1/")
	})

	t.Run("returns error when workflow missing from ref", func(t *testing.T) {
		_, err := GetActionWorkflowByRef(ctx, gitRepo, repo, "nonexistent.yml", git.RefName("refs/heads/feature"))
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrNotExist)
	})
}

func TestToActionWorkflowRun_UsesTriggerEvent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 803})
	run.Repo = repo
	// Scheduled runs keep Event as the registration event (push) and use TriggerEvent as the real trigger.
	run.Event = "push"
	run.TriggerEvent = "schedule"

	apiRun, err := ToActionWorkflowRun(t.Context(), run, nil)
	require.NoError(t, err)
	assert.Equal(t, "schedule", apiRun.Event)
}

func TestToActionWorkflowJob_StepStatusIsIndependentOfJobStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	run := &actions_model.ActionRun{
		ID:            9001,
		RepoID:        2,
		TriggerUserID: 1,
		WorkflowID:    "test.yaml",
		Index:         12345,
		Ref:           "refs/heads/main",
		Status:        actions_model.StatusFailure,
	}
	require.NoError(t, db.Insert(ctx, run))

	task := &actions_model.ActionTask{
		ID:     900102,
		JobID:  9001,
		RepoID: 2,
		Status: actions_model.StatusFailure,
	}
	require.NoError(t, db.Insert(ctx, task))

	job := &actions_model.ActionRunJob{
		ID:      90010203,
		RunID:   9001,
		TaskID:  900102,
		RepoID:  2,
		Name:    "test-job-name",
		Attempt: 1,
		JobID:   "test-job-id",
		Status:  actions_model.StatusFailure,
	}
	require.NoError(t, db.Insert(ctx, job))

	require.NoError(t, db.Insert(ctx,
		&actions_model.ActionTaskStep{TaskID: task.ID, RepoID: 2, Index: 0, Name: "step-success", Status: actions_model.StatusSuccess},
		&actions_model.ActionTaskStep{TaskID: task.ID, RepoID: 2, Index: 1, Name: "step-failure", Status: actions_model.StatusFailure},
	))

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	apiJob, err := ToActionWorkflowJob(ctx, repo, task, job)
	require.NoError(t, err)
	require.Len(t, apiJob.Steps, 2)

	assert.Equal(t, "completed", apiJob.Steps[0].Status, "step 0 status")
	assert.Equal(t, "success", apiJob.Steps[0].Conclusion, "step 0 conclusion (succeeded before the failure)")
	assert.Equal(t, "completed", apiJob.Steps[1].Status, "step 1 status")
	assert.Equal(t, "failure", apiJob.Steps[1].Conclusion, "step 1 conclusion")
}
