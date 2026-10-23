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
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO TestPullRequest_PushToBaseRepo

func TestPullRequest_FormatSquashMergeCommitMessages(t *testing.T) {
	oldest := &git.Commit{CommitMessage: git.CommitMessage{MessageRaw: "commit msg 1"}}
	newest := &git.Commit{CommitMessage: git.CommitMessage{MessageRaw: "commit msg 2\n\nCommit description."}}

	defer test.MockVariableValue(&setting.Repository.PullRequest.DefaultMergeMessageSize, 0)()

	assert.Equal(t, "* commit msg 1\n\n* commit msg 2\n\nCommit description.\n\n", formatSquashMergeCommitMessages([]*git.Commit{newest, oldest}))

	utf8Msg := &git.Commit{CommitMessage: git.CommitMessage{MessageRaw: "🌞"}}
	setting.Repository.PullRequest.DefaultMergeMessageSize = 3
	assert.Equal(t, "* ...\n\n", formatSquashMergeCommitMessages([]*git.Commit{utf8Msg}))
	setting.Repository.PullRequest.DefaultMergeMessageSize = 4
	assert.Equal(t, "* ...\n\n", formatSquashMergeCommitMessages([]*git.Commit{utf8Msg}))
	setting.Repository.PullRequest.DefaultMergeMessageSize = 5
	assert.Equal(t, "* ...\n\n", formatSquashMergeCommitMessages([]*git.Commit{utf8Msg}))
	setting.Repository.PullRequest.DefaultMergeMessageSize = 6
	assert.Equal(t, "* 🌞\n\n", formatSquashMergeCommitMessages([]*git.Commit{utf8Msg}))
}

func TestPullRequest_GetDefaultMergeMessage_InternalTracker(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})

	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	gitRepo, err := git.OpenRepository(t.Context(), pr.BaseRepo)
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
	gitRepo, err := git.OpenRepository(t.Context(), pr.BaseRepo)
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

	oldBaseCommitID, err := git.GetFullCommitID(t.Context(), pr.BaseRepo, pr.BaseBranch)
	require.NoError(t, err)
	oldHeadCommitID, err := git.GetFullCommitID(t.Context(), pr.HeadRepo, pr.HeadBranch)
	require.NoError(t, err)

	baseWorktreePath := filepath.Join(t.TempDir(), "base-worktree")
	runGitCommand(t, "clone", gitrepo.RepoLocalPath(pr.BaseRepo), baseWorktreePath)
	runGitCommand(t, "-C", baseWorktreePath, "config", "user.name", "Gitea Tests")
	runGitCommand(t, "-C", baseWorktreePath, "config", "user.email", "tests@gitea.io")

	regressionFilePath := filepath.Join(baseWorktreePath, "fork-pr-regression.txt")
	require.NoError(t, os.WriteFile(regressionFilePath, []byte("fork PR regression\n"), 0o644))
	runGitCommand(t, "-C", baseWorktreePath, "add", filepath.Base(regressionFilePath))
	runGitCommand(t, "-C", baseWorktreePath, "commit", "-m", "Advance base branch for fork PR regression test")
	runGitCommand(t, "--git-dir", gitrepo.RepoLocalPath(pr.BaseRepo), "fetch", baseWorktreePath, "HEAD:refs/heads/"+pr.BaseBranch)

	newBaseCommitID, err := git.GetFullCommitID(t.Context(), pr.BaseRepo, pr.BaseBranch)
	require.NoError(t, err)
	require.NotEqual(t, oldBaseCommitID, newBaseCommitID)

	headWorktreePath := filepath.Join(t.TempDir(), "head-worktree")
	runGitCommand(t, "clone", gitrepo.RepoLocalPath(pr.HeadRepo), headWorktreePath)
	runGitCommand(t, "-C", headWorktreePath, "checkout", pr.HeadBranch)
	runGitCommand(t, "-C", headWorktreePath, "config", "user.name", "Gitea Tests")
	runGitCommand(t, "-C", headWorktreePath, "config", "user.email", "tests@gitea.io")

	headChangePath := filepath.Join(headWorktreePath, "fork-pr-head-change.txt")
	require.NoError(t, os.WriteFile(headChangePath, []byte("new head content\n"), 0o644))
	runGitCommand(t, "-C", headWorktreePath, "add", filepath.Base(headChangePath))
	runGitCommand(t, "-C", headWorktreePath, "commit", "-m", "Add head change for fork PR regression test")
	runGitCommand(t, "--git-dir", gitrepo.RepoLocalPath(pr.HeadRepo), "fetch", headWorktreePath, "HEAD:refs/heads/"+pr.HeadBranch)

	newHeadCommitID, err := git.GetFullCommitID(t.Context(), pr.HeadRepo, pr.HeadBranch)
	require.NoError(t, err)
	require.NotEqual(t, oldHeadCommitID, newHeadCommitID)

	_, err = git.MergeBase(t.Context(), pr.HeadRepo, newBaseCommitID, newHeadCommitID)
	require.Error(t, err)

	changed, mergeBase, err := checkIfPRContentChanged(t.Context(), pr, oldHeadCommitID, newHeadCommitID)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, oldBaseCommitID, mergeBase)

	mergeBaseAfterFetch, err := git.MergeBase(t.Context(), pr.HeadRepo, newBaseCommitID, newHeadCommitID)
	require.NoError(t, err)
	assert.Equal(t, oldBaseCommitID, mergeBaseAfterFetch)
}

func runGitCommand(t *testing.T, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, output)
}

func TestBuildSquashMergeCommitMessages(t *testing.T) {
	cases := []struct {
		msg       string
		coAuthors []string
		expected  string
	}{
		{"", []string{"the-user"}, "Co-authored-by: the-user\n"},
		{"title", nil, "title"},
		{"title", []string{"the-user"}, "title\n\n---------\n\nCo-authored-by: the-user\n"},
		{"title\n\n", []string{"the-user"}, "title\n\n---------\n\nCo-authored-by: the-user\n"},
		{"title\n\nKey: val", []string{"the-user"}, "title\n\nKey: val\nCo-authored-by: the-user\n"},
		{"title\n\n----\nKey: val", []string{"the-user"}, "title\n\n----\nKey: val\nCo-authored-by: the-user\n"},
		{"title\n\n----\nKey: val\n\n", []string{"the-user"}, "title\n\n----\nKey: val\nCo-authored-by: the-user\n"},

		{"title\n\nbody", nil, "title\n\nbody"},
		{"title\n\nbody", []string{"the-user"}, "title\n\nbody\n\n---------\n\nCo-authored-by: the-user\n"},
		{"title\n\nbody\n\nKey: val", []string{"the-user"}, "title\n\nbody\n\nKey: val\nCo-authored-by: the-user\n"},
		{"title\n\nbody\n\n----\nKey: val", []string{"the-user"}, "title\n\nbody\n\n----\nKey: val\nCo-authored-by: the-user\n"},
	}
	for _, c := range cases {
		msg := buildSquashMergeCommitMessages(c.msg, c.coAuthors)
		assert.Equal(t, c.expected, msg, "msg: %s", c.msg)
	}
}
