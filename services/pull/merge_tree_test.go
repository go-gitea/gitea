// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
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

	pr.BaseBranch, pr.HeadBranch = "test-merge-tree-empty-base", "test-merge-tree-empty-head"
	createEmptyBranches(t, pr.BaseRepo.RepoPath(), pr.BaseBranch, pr.HeadBranch)
	t.Run("Empty-MergeTree", func(t *testing.T) {
		testPullRequestMergeCheck(t, checkPullRequestMergeableByMergeTree, pr, issues_model.PullRequestStatusEmpty, nil, nil)
	})
	t.Run("Empty-TmpRepo", func(t *testing.T) {
		testPullRequestMergeCheck(t, checkPullRequestMergeableByTmpRepo, pr, issues_model.PullRequestStatusEmpty, nil, nil)
	})
}

func createConflictBranches(t *testing.T, repoPath, baseBranch, headBranch string) []string {
	conflictFile := "conflict.txt"
	stdin := fmt.Sprintf(
		`reset refs/heads/%[1]s
from refs/heads/master

commit refs/heads/%[1]s
mark :1
committer Test <test@example.com> 0 +0000
data 17
add conflict file
M 100644 inline %[3]s
data 4
base

commit refs/heads/%[1]s
mark :2
committer Test <test@example.com> 0 +0000
data 11
base change
from :1
M 100644 inline %[3]s
data 11
base change

reset refs/heads/%[2]s
from :1

commit refs/heads/%[2]s
mark :3
committer Test <test@example.com> 0 +0000
data 11
head change
from :1
M 100644 inline %[3]s
data 11
head change
`, baseBranch, headBranch, conflictFile)
	err := gitcmd.NewCommand("fast-import").WithDir(repoPath).WithStdinBytes([]byte(stdin)).RunWithStderr(t.Context())
	require.NoError(t, err)
	return []string{conflictFile}
}

func createEmptyBranches(t *testing.T, repoPath, baseBranch, headBranch string) {
	emptyFile := "empty.txt"
	stdin := fmt.Sprintf(`reset refs/heads/%[1]s
from refs/heads/master

commit refs/heads/%[1]s
mark :1
committer Test <test@example.com> 0 +0000
data 14
add empty file
M 100644 inline %[3]s
data 4
base

reset refs/heads/%[2]s
from :1

commit refs/heads/%[2]s
mark :2
committer Test <test@example.com> 0 +0000
data 17
change empty file
from :1
M 100644 inline %[3]s
data 6
change

commit refs/heads/%[2]s
mark :3
committer Test <test@example.com> 0 +0000
data 17
revert empty file
from :2
M 100644 inline %[3]s
data 4
base
`, baseBranch, headBranch, emptyFile)
	err := gitcmd.NewCommand("fast-import").WithDir(repoPath).WithStdinBytes([]byte(stdin)).RunWithStderr(t.Context())
	require.NoError(t, err)
}
