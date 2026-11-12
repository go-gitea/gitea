// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
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

	// the fixture repos are shared by other tests, restore the branches afterwards
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		assert.NoError(t, git.UpdateRef(ctx, pr.BaseRepo, git.BranchPrefix+pr.BaseBranch, oldBaseCommitID))
		assert.NoError(t, git.UpdateRef(ctx, pr.HeadRepo, git.BranchPrefix+pr.HeadBranch, oldHeadCommitID))
	})

	advanceBranch := func(repo *repo_model.Repository, branch, parentID, fileName string) string {
		require.NoError(t, git.ForceFastImport(t.Context(), repo, []git.FastImportCommit{{
			Ref: git.BranchPrefix + branch, Parent: parentID,
			Files: []git.FastImportFile{{Path: fileName, Content: "content of " + fileName + "\n"}},
		}}))
		commitID, err := git.GetFullCommitID(t.Context(), repo, branch)
		require.NoError(t, err)
		require.NotEqual(t, parentID, commitID)
		return commitID
	}
	newBaseCommitID := advanceBranch(pr.BaseRepo, pr.BaseBranch, oldBaseCommitID, "fork-pr-regression.txt")
	newHeadCommitID := advanceBranch(pr.HeadRepo, pr.HeadBranch, oldHeadCommitID, "fork-pr-head-change.txt")

	// the new base commit only exists in the base repo, so it must be fetched into the head repo
	exist, err := git.IsCommitInRepo(t.Context(), pr.HeadRepo, newBaseCommitID)
	require.NoError(t, err)
	assert.False(t, exist)

	changed, mergeBase, err := checkIfPRContentChanged(t.Context(), pr, oldHeadCommitID, newHeadCommitID)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, oldBaseCommitID, mergeBase)
	exist, err = git.IsCommitInRepo(t.Context(), pr.HeadRepo, newBaseCommitID)
	require.NoError(t, err)
	assert.True(t, exist)
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
