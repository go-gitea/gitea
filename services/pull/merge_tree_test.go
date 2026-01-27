// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"os"
	"path/filepath"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_testPullRequestMergeTree(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	// pull 2 is mergeable, set to conflicted to see if the function updates it correctly
	pr.Status = issues_model.PullRequestStatusConflict
	pr.ConflictedFiles = []string{"old_file.go"}
	pr.ChangedProtectedFiles = []string{"protected_file.go"}

	err := testPullRequestMergeTree(t.Context(), pr)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusMergeable, pr.Status)
	assert.Empty(t, pr.ConflictedFiles)
	assert.Empty(t, pr.ChangedProtectedFiles)
	assert.NotEmpty(t, pr.MergeBase)
	assert.NotEmpty(t, pr.HeadCommitID)
}

func Test_testPullRequestTmpRepoBranchMergeable(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	// pull 3 is mergeable, set to conflicted to see if the function updates it correctly
	pr.Status = issues_model.PullRequestStatusConflict
	pr.ConflictedFiles = []string{"old_file.go"}
	pr.ChangedProtectedFiles = []string{"protected_file.go"}

	err := testPullRequestTmpRepoBranchMergeable(t.Context(), pr)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusMergeable, pr.Status)
	assert.Empty(t, pr.ConflictedFiles)
	assert.Empty(t, pr.ChangedProtectedFiles)
	assert.NotEmpty(t, pr.MergeBase)
	assert.NotEmpty(t, pr.HeadCommitID)
}

func Test_testPullRequestMergeTree_Conflict(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	baseBranch := "test-merge-tree-conflict-base"
	headBranch := "test-merge-tree-conflict-head"
	conflictFiles := createConflictBranches(t, pr.BaseRepo.RepoPath(), baseBranch, headBranch)

	pr.BaseBranch = baseBranch
	pr.HeadBranch = headBranch
	pr.Status = issues_model.PullRequestStatusMergeable
	pr.ConflictedFiles = nil
	pr.ChangedProtectedFiles = nil

	err := testPullRequestMergeTree(t.Context(), pr)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusConflict, pr.Status)
	assert.Contains(t, pr.ConflictedFiles, conflictFiles[0])
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

	err := testPullRequestMergeTree(t.Context(), pr)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusEmpty, pr.Status)
	assert.Empty(t, pr.ConflictedFiles)
}

func Test_testPullRequestTmpRepoBranchMergeable_Conflict(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	baseBranch := "test-tmp-conflict-base"
	headBranch := "test-tmp-conflict-head"
	conflictFiles := createConflictBranches(t, pr.BaseRepo.RepoPath(), baseBranch, headBranch)

	pr.BaseBranch = baseBranch
	pr.HeadBranch = headBranch
	pr.Status = issues_model.PullRequestStatusMergeable
	pr.ConflictedFiles = nil
	pr.ChangedProtectedFiles = nil

	err := testPullRequestTmpRepoBranchMergeable(t.Context(), pr)
	assert.NoError(t, err)
	assert.Equal(t, issues_model.PullRequestStatusConflict, pr.Status)
	assert.Contains(t, pr.ConflictedFiles, conflictFiles[0])
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

	err := testPullRequestTmpRepoBranchMergeable(t.Context(), pr)
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
