// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPullRequestMergeCheck(t *testing.T,
	targetFunc func(ctx context.Context, pr *issues_model.PullRequest) error,
	pr *issues_model.PullRequest,
	expectedStatus issues_model.PullRequestStatus,
	expectedConflictedFiles []string,
	expectedChangedProtectedFiles []string,
) {
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))
	pr.Status = issues_model.PullRequestStatusChecking
	pr.ConflictedFiles = []string{"unrelated-conflicted-file"}
	pr.ChangedProtectedFiles = []string{"unrelated-protected-file"}
	pr.MergeBase = ""
	pr.HeadCommitID = ""
	err := targetFunc(t.Context(), pr)
	require.NoError(t, err)
	assert.Equal(t, expectedStatus, pr.Status)
	assert.Equal(t, expectedConflictedFiles, pr.ConflictedFiles)
	assert.Equal(t, expectedChangedProtectedFiles, pr.ChangedProtectedFiles)
	assert.NotEmpty(t, pr.MergeBase)
	assert.NotEmpty(t, pr.HeadCommitID)
}

func TestPullRequestMergeable(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	t.Run("NoConflict-MergeTree", func(t *testing.T) {
		testPullRequestMergeCheck(t, checkPullRequestMergeableByMergeTree, pr, issues_model.PullRequestStatusMergeable, nil, nil)
	})
	t.Run("NoConflict-TmpRepo", func(t *testing.T) {
		testPullRequestMergeCheck(t, checkPullRequestMergeableByTmpRepo, pr, issues_model.PullRequestStatusMergeable, nil, nil)
	})

	pr.BaseBranch, pr.HeadBranch = "test-merge-tree-conflict-base", "test-merge-tree-conflict-head"
	conflictFiles := createConflictBranches(t, pr.BaseRepo.RepoPath(), pr.BaseBranch, pr.HeadBranch)
	t.Run("Conflict-MergeTree", func(t *testing.T) {
		testPullRequestMergeCheck(t, checkPullRequestMergeableByMergeTree, pr, issues_model.PullRequestStatusConflict, conflictFiles, nil)
	})
	t.Run("Conflict-TmpRepo", func(t *testing.T) {
		testPullRequestMergeCheck(t, checkPullRequestMergeableByTmpRepo, pr, issues_model.PullRequestStatusConflict, conflictFiles, nil)
	})
}

func Test_testPullRequestMergeTree_Empty(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	baseBranch := "test-merge-tree-empty-base"
	headBranch := "test-merge-tree-empty-head"
	createEmptyBranches(t, pr.BaseRepo.RepoPath(), baseBranch, headBranch)

	pr.BaseBranch = baseBranch
	pr.HeadBranch = headBranch
	pr.Status = issues_model.PullRequestStatusMergeable
	pr.ConflictedFiles = []string{"old_file.go"}
	pr.ChangedProtectedFiles = []string{"protected_file.go"}

	err := checkPullRequestMergeableByMergeTree(t.Context(), pr)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusEmpty, pr.Status)
	assert.Empty(t, pr.ConflictedFiles)
}

func Test_testPullRequestTmpRepoBranchMergeable_Empty(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	baseBranch := "test-tmp-empty-base"
	headBranch := "test-tmp-empty-head"
	createEmptyBranches(t, pr.BaseRepo.RepoPath(), baseBranch, headBranch)

	pr.BaseBranch = baseBranch
	pr.HeadBranch = headBranch
	pr.Status = issues_model.PullRequestStatusMergeable
	pr.ConflictedFiles = []string{"old_file.go"}
	pr.ChangedProtectedFiles = nil

	err := checkPullRequestMergeableByTmpRepo(t.Context(), pr)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusEmpty, pr.Status)
	assert.Empty(t, pr.ConflictedFiles)
}

func createConflictBranches(t *testing.T, repoPath, baseBranch, headBranch string) []string {
	t.Helper()

	disableRepoHooks(t, repoPath)
	workDir := cloneRepoForTest(t, repoPath)
	conflictFile := "conflict.txt"

	assert.NoError(t, gitcmd.NewCommand("checkout", "-B").AddDynamicArguments(baseBranch, "master").WithDir(workDir).Run(t.Context()))
	writeFile(t, workDir, conflictFile, "base")
	assert.NoError(t, gitcmd.NewCommand("add").AddDynamicArguments(conflictFile).WithDir(workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments("add conflict file").WithDir(workDir).Run(t.Context()))

	assert.NoError(t, gitcmd.NewCommand("checkout", "-B").AddDynamicArguments(headBranch, baseBranch).WithDir(workDir).Run(t.Context()))

	assert.NoError(t, gitcmd.NewCommand("checkout").AddDynamicArguments(baseBranch).WithDir(workDir).Run(t.Context()))
	writeFile(t, workDir, conflictFile, "base change")
	assert.NoError(t, gitcmd.NewCommand("add").AddDynamicArguments(conflictFile).WithDir(workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments("base change").WithDir(workDir).Run(t.Context()))

	assert.NoError(t, gitcmd.NewCommand("checkout").AddDynamicArguments(headBranch).WithDir(workDir).Run(t.Context()))
	writeFile(t, workDir, conflictFile, "head change")
	assert.NoError(t, gitcmd.NewCommand("add").AddDynamicArguments(conflictFile).WithDir(workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments("head change").WithDir(workDir).Run(t.Context()))

	assert.NoError(t, gitcmd.NewCommand("push", "origin").AddDynamicArguments(baseBranch, headBranch).WithDir(workDir).Run(t.Context()))
	return []string{conflictFile}
}

func createEmptyBranches(t *testing.T, repoPath, baseBranch, headBranch string) {
	t.Helper()

	disableRepoHooks(t, repoPath)
	workDir := cloneRepoForTest(t, repoPath)
	emptyFile := "empty.txt"

	assert.NoError(t, gitcmd.NewCommand("checkout", "-B").AddDynamicArguments(baseBranch, "master").WithDir(workDir).Run(t.Context()))
	writeFile(t, workDir, emptyFile, "base")
	assert.NoError(t, gitcmd.NewCommand("add").AddDynamicArguments(emptyFile).WithDir(workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments("add empty file").WithDir(workDir).Run(t.Context()))

	assert.NoError(t, gitcmd.NewCommand("checkout", "-B").AddDynamicArguments(headBranch, baseBranch).WithDir(workDir).Run(t.Context()))
	writeFile(t, workDir, emptyFile, "change")
	assert.NoError(t, gitcmd.NewCommand("add").AddDynamicArguments(emptyFile).WithDir(workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments("change empty file").WithDir(workDir).Run(t.Context()))

	writeFile(t, workDir, emptyFile, "base")
	assert.NoError(t, gitcmd.NewCommand("add").AddDynamicArguments(emptyFile).WithDir(workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("commit", "-m").AddDynamicArguments("revert empty file").WithDir(workDir).Run(t.Context()))

	assert.NoError(t, gitcmd.NewCommand("push", "origin").AddDynamicArguments(baseBranch, headBranch).WithDir(workDir).Run(t.Context()))
}

func cloneRepoForTest(t *testing.T, repoPath string) string {
	t.Helper()

	workDir := t.TempDir()
	assert.NoError(t, gitcmd.NewCommand("clone").AddDynamicArguments(repoPath, workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("config", "user.name").AddDynamicArguments("Test User").WithDir(workDir).Run(t.Context()))
	assert.NoError(t, gitcmd.NewCommand("config", "user.email").AddDynamicArguments("test@example.com").WithDir(workDir).Run(t.Context()))
	return workDir
}

func disableRepoHooks(t *testing.T, repoPath string) {
	t.Helper()

	configPath := filepath.Join(repoPath, "config")
	assert.NoError(t, gitcmd.NewCommand("config", "-f").AddDynamicArguments(configPath, "core.hooksPath", "/dev/null").Run(t.Context()))
}

func writeFile(t *testing.T, workDir, filename, content string) {
	t.Helper()

	filePath := filepath.Join(workDir, filename)
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))
}
