// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"fmt"
	"strings"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupConflictPR creates two branches that have a content conflict and
// returns a PullRequest pointing at them.  The conflict is in "conflict.txt":
// both branches diverge from a common ancestor and each makes an incompatible
// change to the same line.
func setupConflictPR(t *testing.T) (pr *issues_model.PullRequest, conflictFile string) {
	t.Helper()
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	require.NoError(t, pr.LoadHeadRepo(t.Context()))

	conflictFile = "conflict.txt"
	baseBranch := fmt.Sprintf("test-cr-base-%s", t.Name())
	headBranch := fmt.Sprintf("test-cr-head-%s", t.Name())
	// Sanitize branch names (test names may contain '/')
	baseBranch = strings.ReplaceAll(baseBranch, "/", "-")
	headBranch = strings.ReplaceAll(headBranch, "/", "-")

	// Fast-import: common ancestor commit, then diverging changes on each branch.
	repoPath := pr.BaseRepo.RepoPath()
	fastImport := fmt.Sprintf(`reset refs/heads/%[1]s
from refs/heads/master

commit refs/heads/%[1]s
mark :1
committer Test <test@example.com> 0 +0000
data 17
add conflict.txt
M 100644 inline %[3]s
data 8
ancestor

commit refs/heads/%[1]s
mark :2
committer Test <test@example.com> 0 +0000
data 16
base branch edit
from :1
M 100644 inline %[3]s
data 12
base version

reset refs/heads/%[2]s
from :1

commit refs/heads/%[2]s
mark :3
committer Test <test@example.com> 0 +0000
data 16
head branch edit
from :1
M 100644 inline %[3]s
data 12
head version
`, baseBranch, headBranch, conflictFile)
	require.NoError(t, gitcmd.NewCommand("fast-import").WithDir(repoPath).
		WithStdinBytes([]byte(fastImport)).RunWithStderr(t.Context()))

	pr.BaseBranch = baseBranch
	pr.HeadBranch = headBranch
	// base == head repo for PR ID 2
	pr.HeadRepoID = pr.BaseRepoID
	pr.HeadRepo = pr.BaseRepo
	pr.Status = issues_model.PullRequestStatusConflict
	pr.ConflictedFiles = []string{conflictFile}
	pr.MergeBase = ""
	pr.HeadCommitID = ""
	return pr, conflictFile
}

// TestGetConflictedFileContent verifies that the function returns content with
// standard git conflict markers when the two branches have incompatible changes.
func TestGetConflictedFileContent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pr, conflictFile := setupConflictPR(t)
	_ = conflictFile

	content, err := GetConflictedFileContent(t.Context(), pr, conflictFile)
	require.NoError(t, err)
	assert.NotEmpty(t, content)

	// The output must contain the git conflict delimiters.
	assert.Contains(t, content, "<<<<<<<", "expected opening conflict marker")
	assert.Contains(t, content, "=======", "expected conflict separator")
	assert.Contains(t, content, ">>>>>>>", "expected closing conflict marker")

	// Labels come from the branch names.
	assert.Contains(t, content, pr.HeadBranch, "<<<<<<< label should be head branch")
	assert.Contains(t, content, pr.BaseBranch, ">>>>>>> label should be base branch")

	// Both sides' content must appear.
	assert.Contains(t, content, "head version")
	assert.Contains(t, content, "base version")
}

// TestGetConflictedFileContentNotConflicted verifies that an error is returned
// when the requested file does not exist on one of the branches.
func TestGetConflictedFileContentNotConflicted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pr, _ := setupConflictPR(t)

	_, err := GetConflictedFileContent(t.Context(), pr, "nonexistent.txt")
	assert.Error(t, err, "should fail for a file not present in the branches")
}

// TestCommitConflictResolution verifies that CommitConflictResolution produces
// a proper merge commit and that the PR conflict check subsequently reports the
// PR as mergeable.
func TestCommitConflictResolution(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pr, conflictFile := setupConflictPR(t)

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Resolve the conflict by keeping content from both sides.
	resolved := []ResolvedFile{
		{Path: conflictFile, Content: "resolved content\n"},
	}
	commitMsg := "Resolve merge conflicts"
	commitEnv := buildTestCommitEnv(doer)

	err := CommitConflictResolution(t.Context(), pr, doer, resolved, commitMsg, commitEnv)
	require.NoError(t, err)

	// After the merge commit, the conflict check must pass.
	pr.Status = issues_model.PullRequestStatusChecking
	pr.ConflictedFiles = nil
	pr.MergeBase = ""
	pr.HeadCommitID = ""
	require.NoError(t, checkPullRequestMergeableByTmpRepo(t.Context(), pr))
	assert.Equal(t, issues_model.PullRequestStatusMergeable, pr.Status,
		"PR should be mergeable after conflict resolution merge commit")
	assert.Empty(t, pr.ConflictedFiles)
}

// TestCommitConflictResolutionMergeCommitParents verifies that the resulting
// commit has exactly two parents: the old head tip and the base branch tip.
func TestCommitConflictResolutionMergeCommitParents(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pr, conflictFile := setupConflictPR(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Record the tip SHAs before the resolution commit.
	repoPath := pr.BaseRepo.RepoPath()
	oldHeadTip, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(pr.HeadBranch).
		WithDir(repoPath).RunStdString(t.Context())
	require.NoError(t, err)
	oldHeadTip = strings.TrimSpace(oldHeadTip)

	baseTip, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(pr.BaseBranch).
		WithDir(repoPath).RunStdString(t.Context())
	require.NoError(t, err)
	baseTip = strings.TrimSpace(baseTip)

	require.NoError(t, CommitConflictResolution(
		t.Context(), pr, doer,
		[]ResolvedFile{{Path: conflictFile, Content: "resolved\n"}},
		"resolve",
		buildTestCommitEnv(doer),
	))

	// The new head tip must have two parents.
	newHeadTip, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(pr.HeadBranch).
		WithDir(repoPath).RunStdString(t.Context())
	require.NoError(t, err)
	newHeadTip = strings.TrimSpace(newHeadTip)
	assert.NotEqual(t, oldHeadTip, newHeadTip, "head tip should advance after merge commit")

	parents, _, err := gitcmd.NewCommand("log", "--pretty=%P", "-1").
		AddDynamicArguments(newHeadTip).
		WithDir(repoPath).RunStdString(t.Context())
	require.NoError(t, err)
	parentList := strings.Fields(strings.TrimSpace(parents))
	require.Len(t, parentList, 2, "merge commit must have exactly two parents")
	assert.Equal(t, oldHeadTip, parentList[0], "first parent should be the old head tip")
	assert.Equal(t, baseTip, parentList[1], "second parent should be the base branch tip")
}

func buildTestCommitEnv(doer *user_model.User) []string {
	sig := doer.NewGitSig()
	return []string{
		"GIT_AUTHOR_NAME=" + sig.Name,
		"GIT_AUTHOR_EMAIL=" + sig.Email,
		"GIT_AUTHOR_DATE=2025-01-01T00:00:00+00:00",
		"GIT_COMMITTER_NAME=" + sig.Name,
		"GIT_COMMITTER_EMAIL=" + sig.Email,
		"GIT_COMMITTER_DATE=2025-01-01T00:00:00+00:00",
	}
}
