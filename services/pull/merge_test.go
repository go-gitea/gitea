// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_expandDefaultMergeMessage(t *testing.T) {
	type args struct {
		template string
		vars     map[string]string
	}
	tests := []struct {
		name     string
		args     args
		want     string
		wantBody string
	}{
		{
			name: "single line",
			args: args{
				template: "Merge ${PullRequestTitle}",
				vars: map[string]string{
					"PullRequestTitle":       "PullRequestTitle",
					"PullRequestDescription": "Pull\nRequest\nDescription\n",
				},
			},
			want:     "Merge PullRequestTitle",
			wantBody: "",
		},
		{
			name: "multiple lines",
			args: args{
				template: "Merge ${PullRequestTitle}\nDescription:\n\n${PullRequestDescription}\n",
				vars: map[string]string{
					"PullRequestTitle":       "PullRequestTitle",
					"PullRequestDescription": "Pull\nRequest\nDescription\n",
				},
			},
			want:     "Merge PullRequestTitle",
			wantBody: "Description:\n\nPull\nRequest\nDescription\n",
		},
		{
			name: "leading newlines",
			args: args{
				template: "\n\n\nMerge ${PullRequestTitle}\n\n\nDescription:\n\n${PullRequestDescription}\n",
				vars: map[string]string{
					"PullRequestTitle":       "PullRequestTitle",
					"PullRequestDescription": "Pull\nRequest\nDescription\n",
				},
			},
			want:     "Merge PullRequestTitle",
			wantBody: "Description:\n\nPull\nRequest\nDescription\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := expandDefaultMergeMessage(tt.args.template, tt.args.vars)
			assert.Equalf(t, tt.want, got, "expandDefaultMergeMessage(%v, %v)", tt.args.template, tt.args.vars)
			assert.Equalf(t, tt.wantBody, got1, "expandDefaultMergeMessage(%v, %v)", tt.args.template, tt.args.vars)
		})
	}
}

func TestAddCommitMessageTailer(t *testing.T) {
	// add tailer for empty message
	assert.Equal(t, "\n\nTest-tailer: TestValue", AddCommitMessageTailer("", "Test-tailer", "TestValue"))

	// add tailer for message without newlines
	assert.Equal(t, "title\n\nTest-tailer: TestValue", AddCommitMessageTailer("title", "Test-tailer", "TestValue"))
	assert.Equal(t, "title\n\nNot tailer: xxx\n\nTest-tailer: TestValue", AddCommitMessageTailer("title\n\nNot tailer: xxx", "Test-tailer", "TestValue"))
	assert.Equal(t, "title\n\nNotTailer: xxx\n\nTest-tailer: TestValue", AddCommitMessageTailer("title\n\nNotTailer: xxx", "Test-tailer", "TestValue"))
	assert.Equal(t, "title\n\nnot-tailer: xxx\n\nTest-tailer: TestValue", AddCommitMessageTailer("title\n\nnot-tailer: xxx", "Test-tailer", "TestValue"))

	// add tailer for message with one EOL
	assert.Equal(t, "title\n\nTest-tailer: TestValue", AddCommitMessageTailer("title\n", "Test-tailer", "TestValue"))

	// add tailer for message with two EOLs
	assert.Equal(t, "title\n\nTest-tailer: TestValue", AddCommitMessageTailer("title\n\n", "Test-tailer", "TestValue"))

	// add tailer for message with existing tailer (won't duplicate)
	assert.Equal(t, "title\n\nTest-tailer: TestValue", AddCommitMessageTailer("title\n\nTest-tailer: TestValue", "Test-tailer", "TestValue"))
	assert.Equal(t, "title\n\nTest-tailer: TestValue\n", AddCommitMessageTailer("title\n\nTest-tailer: TestValue\n", "Test-tailer", "TestValue"))

	// add tailer for message with existing tailer and different value (will append)
	assert.Equal(t, "title\n\nTest-tailer: v1\nTest-tailer: v2", AddCommitMessageTailer("title\n\nTest-tailer: v1", "Test-tailer", "v2"))
	assert.Equal(t, "title\n\nTest-tailer: v1\nTest-tailer: v2", AddCommitMessageTailer("title\n\nTest-tailer: v1\n", "Test-tailer", "v2"))
}

func TestResolveMergeMessageTemplate(t *testing.T) {
	t.Run("NoDefault", func(t *testing.T) {
		repo, err := git.ForceFastImportWithInit(t.Context(), t.TempDir(), []git.FastImportCommit{
			{Ref: "refs/heads/master", Files: []git.FastImportFile{
				{Path: ".gitea/default_merge_message/REBASE_TEMPLATE.md", Content: "rebase template"},
			}},
		})
		require.NoError(t, err)
		gitRepo, err := git.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		defer gitRepo.Close()

		commit, err := gitRepo.GetBranchCommit(t.Context(), "master")
		require.NoError(t, err)
		tmpl, err := resolveMergeMessageTemplate(t.Context(), gitRepo, commit, "merge")
		assert.NoError(t, err)
		assert.Equal(t, "", tmpl)
		tmpl, err = resolveMergeMessageTemplate(t.Context(), gitRepo, commit, "rebase")
		assert.NoError(t, err)
		assert.Equal(t, "rebase template", tmpl)
	})
	t.Run("WithDefault", func(t *testing.T) {
		repo, err := git.ForceFastImportWithInit(t.Context(), t.TempDir(), []git.FastImportCommit{
			{Ref: "refs/heads/master", Files: []git.FastImportFile{
				{Path: ".gitea/default_merge_message/DEFAULT_TEMPLATE.md", Content: "default template"},
				{Path: ".gitea/default_merge_message/REBASE_TEMPLATE.md", Content: "rebase template"},
			}},
		})
		require.NoError(t, err)
		gitRepo, err := git.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		defer gitRepo.Close()

		commit, err := gitRepo.GetBranchCommit(t.Context(), "master")
		require.NoError(t, err)
		tmpl, err := resolveMergeMessageTemplate(t.Context(), gitRepo, commit, "merge")
		assert.NoError(t, err)
		assert.Equal(t, "default template", tmpl)
		tmpl, err = resolveMergeMessageTemplate(t.Context(), gitRepo, commit, "rebase")
		assert.NoError(t, err)
		assert.Equal(t, "rebase template", tmpl)
	})
}

func TestSyncMergedState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assert.False(t, pr.HasMerged)
	assert.Equal(t, issues_model.PullRequestMergeStateNone, pr.MergeState)

	// an auto merge schedule must not survive the merge it asked for
	require.NoError(t, pull_model.ScheduleAutoMerge(t.Context(), doer, pr.ID, repo_model.MergeStyleSquash, "squash merge a pr", false))

	require.NoError(t, markPullRequestMerging(t.Context(), pr, doer.ID, false))

	mergeCommitID := "0123456789abcdef0123456789abcdef01234567"
	require.NoError(t, syncMergedState(t.Context(), pr.ID, doer, mergeCommitID))

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.True(t, pr.HasMerged)
	assert.Equal(t, issues_model.PullRequestMergeStateNone, pr.MergeState)
	assert.Equal(t, mergeCommitID, pr.MergedCommitID)
	assert.NotZero(t, pr.MergedUnix)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
	assert.True(t, issue.IsClosed)
	unittest.AssertNotExistsBean(t, &pull_model.AutoMerge{PullID: pr.ID})
}

func TestMarkAndClearPullRequestMerging(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.Equal(t, issues_model.PullRequestMergeStateNone, pr.MergeState)

	require.NoError(t, markPullRequestMerging(t.Context(), pr, 2, true))
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.Equal(t, issues_model.PullRequestMergeStateAutoMerging, pr.MergeState)
	assert.EqualValues(t, 2, pr.MergerID)
	assert.Zero(t, pr.MergedUnix, "an unmerged pull request must not get a merge timestamp")

	// a second merger must not be able to take over a merge that is already in flight
	assert.ErrorIs(t, markPullRequestMerging(t.Context(), pr, 3, false), ErrIsMerging)

	require.NoError(t, setPullRequestMergingCommitID(t.Context(), pr, "0123456789abcdef0123456789abcdef01234567"))
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.Equal(t, "0123456789abcdef0123456789abcdef01234567", pr.MergedCommitID)
	assert.False(t, pr.HasMerged)
	assert.Zero(t, pr.MergedUnix)

	mergingIDs, err := issues_model.GetPullRequestIDsByMerging(t.Context())
	require.NoError(t, err)
	assert.Contains(t, mergingIDs, pr.ID)

	require.NoError(t, clearPullRequestMerging(t.Context(), pr))
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.Equal(t, issues_model.PullRequestMergeStateNone, pr.MergeState)
	assert.EqualValues(t, 0, pr.MergerID)
	assert.Empty(t, pr.MergedCommitID)
	assert.Zero(t, pr.MergedUnix)

	mergingIDs, err = issues_model.GetPullRequestIDsByMerging(t.Context())
	require.NoError(t, err)
	assert.NotContains(t, mergingIDs, pr.ID)
}

func TestRecoverMergingPullRequestNotMerged(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, markPullRequestMerging(t.Context(), pr, 2, false))
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})

	// no merge commit was recorded, so the merge never got as far as the push and must be released for a retry
	merged, err := recoverMergingPullRequest(t.Context(), pr)
	require.NoError(t, err)
	assert.False(t, merged)

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.False(t, pr.HasMerged)
	assert.Equal(t, issues_model.PullRequestMergeStateNone, pr.MergeState)
	assert.EqualValues(t, 0, pr.MergerID)
}

func TestRecoverMergingPullRequestMergeLanded(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, markPullRequestMerging(t.Context(), pr, 2, true))
	// the merge reached the base branch but the result was never recorded, which is what recovery has to notice
	baseBranchCommitID := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	require.NoError(t, setPullRequestMergingCommitID(t.Context(), pr, baseBranchCommitID))

	merged, err := recoverMergingPullRequest(t.Context(), pr)
	require.NoError(t, err)
	assert.True(t, merged)

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.True(t, pr.HasMerged)
	assert.Equal(t, issues_model.PullRequestMergeStateNone, pr.MergeState)
	assert.Equal(t, baseBranchCommitID, pr.MergedCommitID)
	assert.NotZero(t, pr.MergedUnix)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
	assert.True(t, issue.IsClosed)
}

func TestRecoverMergingPullRequestCommitNotPushed(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, markPullRequestMerging(t.Context(), pr, 2, false))
	// a merge commit that was recorded but never made it into the base branch
	require.NoError(t, setPullRequestMergingCommitID(t.Context(), pr, "0123456789abcdef0123456789abcdef01234567"))

	merged, err := recoverMergingPullRequest(t.Context(), pr)
	require.NoError(t, err)
	assert.False(t, merged)

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.False(t, pr.HasMerged)
	assert.Equal(t, issues_model.PullRequestMergeStateNone, pr.MergeState)
	assert.Empty(t, pr.MergedCommitID)
}
