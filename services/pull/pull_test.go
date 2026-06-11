// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO TestPullRequest_PushToBaseRepo

func TestPullRequest_CommitMessageTrailersPattern(t *testing.T) {
	// Not a valid trailer section
	assert.False(t, commitMessageTrailersPattern.MatchString(""))
	assert.False(t, commitMessageTrailersPattern.MatchString("No trailer."))
	assert.False(t, commitMessageTrailersPattern.MatchString("Signed-off-by: Bob <bob@example.com>\nNot a trailer due to following text."))
	assert.False(t, commitMessageTrailersPattern.MatchString("Message body not correctly separated from trailer section by empty line.\nSigned-off-by: Bob <bob@example.com>"))
	// Valid trailer section
	assert.True(t, commitMessageTrailersPattern.MatchString("Signed-off-by: Bob <bob@example.com>"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Signed-off-by: Bob <bob@example.com>\nOther-Trailer: Value"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Message body correctly separated from trailer section by empty line.\n\nSigned-off-by: Bob <bob@example.com>"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Multiple trailers.\n\nSigned-off-by: Bob <bob@example.com>\nOther-Trailer: Value"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Newline after trailer section.\n\nSigned-off-by: Bob <bob@example.com>\n"))
	assert.True(t, commitMessageTrailersPattern.MatchString("No space after colon is accepted.\n\nSigned-off-by:Bob <bob@example.com>"))
	assert.True(t, commitMessageTrailersPattern.MatchString("Additional whitespace is accepted.\n\nSigned-off-by \t :  \tBob   <bob@example.com>   "))
	assert.True(t, commitMessageTrailersPattern.MatchString("Folded value.\n\nFolded-trailer: This is\n a folded\n   trailer value\nOther-Trailer: Value"))
}

func TestPullRequest_GetDefaultMergeMessage_InternalTracker(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})

	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	mergeMessage, _, err := GetDefaultMergeMessage(t.Context(), gitRepo, pr, "")
	assert.NoError(t, err)
	assert.Equal(t, "Merge pull request 'issue3' (#3) from branch2 into master", mergeMessage)

	pr.BaseRepoID = 1
	pr.HeadRepoID = 2
	mergeMessage, _, err = GetDefaultMergeMessage(t.Context(), gitRepo, pr, "")
	assert.NoError(t, err)
	assert.Equal(t, "Merge pull request 'issue3' (#3) from user2/repo1:branch2 into master", mergeMessage)
}

func TestPullRequest_GetDefaultMergeMessage_ExternalTracker(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	externalTracker := repo_model.RepoUnit{
		Type: unit.TypeExternalTracker,
		Config: &repo_model.ExternalTrackerConfig{
			ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
		},
	}
	baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	baseRepo.Units = []*repo_model.RepoUnit{&externalTracker}

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2, BaseRepo: baseRepo})

	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	mergeMessage, _, err := GetDefaultMergeMessage(t.Context(), gitRepo, pr, "")
	assert.NoError(t, err)

	assert.Equal(t, "Merge pull request 'issue3' (!3) from branch2 into master", mergeMessage)

	pr.BaseRepoID = 1
	pr.HeadRepoID = 2
	pr.BaseRepo = nil
	pr.HeadRepo = nil
	mergeMessage, _, err = GetDefaultMergeMessage(t.Context(), gitRepo, pr, "")
	assert.NoError(t, err)

	assert.Equal(t, "Merge pull request 'issue3' (#3) from user2/repo2:branch2 into master", mergeMessage)
}

func TestCheckIfPRContentChanged_ForkPR(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
	require.NoError(t, pr.LoadBaseRepo(t.Context()))
	require.NoError(t, pr.LoadHeadRepo(t.Context()))

	oldBaseCommitID, err := gitrepo.GetFullCommitID(t.Context(), pr.BaseRepo, pr.BaseBranch)
	require.NoError(t, err)
	oldHeadCommitID, err := gitrepo.GetFullCommitID(t.Context(), pr.HeadRepo, pr.HeadBranch)
	require.NoError(t, err)

	baseWorktreePath := filepath.Join(t.TempDir(), "base-worktree")
	runGitCommand(t, "clone", pr.BaseRepo.RepoPath(), baseWorktreePath)
	runGitCommand(t, "-C", baseWorktreePath, "config", "user.name", "Gitea Tests")
	runGitCommand(t, "-C", baseWorktreePath, "config", "user.email", "tests@gitea.io")

	regressionFilePath := filepath.Join(baseWorktreePath, "fork-pr-regression.txt")
	require.NoError(t, os.WriteFile(regressionFilePath, []byte("fork PR regression\n"), 0o644))
	runGitCommand(t, "-C", baseWorktreePath, "add", filepath.Base(regressionFilePath))
	runGitCommand(t, "-C", baseWorktreePath, "commit", "-m", "Advance base branch for fork PR regression test")
	runGitCommand(t, "--git-dir", pr.BaseRepo.RepoPath(), "fetch", baseWorktreePath, "HEAD:refs/heads/"+pr.BaseBranch)

	newBaseCommitID, err := gitrepo.GetFullCommitID(t.Context(), pr.BaseRepo, pr.BaseBranch)
	require.NoError(t, err)
	require.NotEqual(t, oldBaseCommitID, newBaseCommitID)

	headWorktreePath := filepath.Join(t.TempDir(), "head-worktree")
	runGitCommand(t, "clone", pr.HeadRepo.RepoPath(), headWorktreePath)
	runGitCommand(t, "-C", headWorktreePath, "checkout", pr.HeadBranch)
	runGitCommand(t, "-C", headWorktreePath, "config", "user.name", "Gitea Tests")
	runGitCommand(t, "-C", headWorktreePath, "config", "user.email", "tests@gitea.io")

	headChangePath := filepath.Join(headWorktreePath, "fork-pr-head-change.txt")
	require.NoError(t, os.WriteFile(headChangePath, []byte("new head content\n"), 0o644))
	runGitCommand(t, "-C", headWorktreePath, "add", filepath.Base(headChangePath))
	runGitCommand(t, "-C", headWorktreePath, "commit", "-m", "Add head change for fork PR regression test")
	runGitCommand(t, "--git-dir", pr.HeadRepo.RepoPath(), "fetch", headWorktreePath, "HEAD:refs/heads/"+pr.HeadBranch)

	newHeadCommitID, err := gitrepo.GetFullCommitID(t.Context(), pr.HeadRepo, pr.HeadBranch)
	require.NoError(t, err)
	require.NotEqual(t, oldHeadCommitID, newHeadCommitID)

	_, err = gitrepo.MergeBase(t.Context(), pr.HeadRepo, newBaseCommitID, newHeadCommitID)
	require.Error(t, err)

	changed, mergeBase, err := checkIfPRContentChanged(t.Context(), pr, oldHeadCommitID, newHeadCommitID)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, oldBaseCommitID, mergeBase)

	mergeBaseAfterFetch, err := gitrepo.MergeBase(t.Context(), pr.HeadRepo, newBaseCommitID, newHeadCommitID)
	require.NoError(t, err)
	assert.Equal(t, oldBaseCommitID, mergeBaseAfterFetch)
}

func runGitCommand(t *testing.T, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, output)
}
