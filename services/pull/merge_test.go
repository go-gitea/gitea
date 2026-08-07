// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetMergedMarksAlreadyClosedPullRequest(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, pr.LoadIssue(ctx))

	pr.Issue.IsClosed = true
	_, err := db.GetEngine(ctx).ID(pr.Issue.ID).Cols("is_closed").Update(pr.Issue)
	require.NoError(t, err)

	merger := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	merged, err := SetMerged(ctx, pr, "0123456789012345678901234567890123456789", timeutil.TimeStampNow(), merger, issues_model.PullRequestStatusManuallyMerged)
	require.NoError(t, err)
	require.True(t, merged)

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.True(t, pr.HasMerged)
	assert.Equal(t, "0123456789012345678901234567890123456789", pr.MergedCommitID)
	assert.Equal(t, merger.ID, pr.MergerID)

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: pr.IssueID})
	assert.True(t, issue.IsClosed)
}

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
