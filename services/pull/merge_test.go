// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"strings"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_expandDefaultMergeMessage(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantBody string
	}{
		{
			name:     "single line",
			template: "Merge ${PullRequestTitle}",
			wantBody: "",
		},
		{
			name:     "multiple lines",
			template: "Merge ${PullRequestTitle}\nDescription:\n\n${PullRequestDescription}\n",
			wantBody: "Description:\n\nPull\nRequest\nDescription\n",
		},
		{
			name:     "leading newlines",
			template: "\n\n\nMerge ${PullRequestTitle}\n\n\nDescription:\n\n${PullRequestDescription}\n",
			wantBody: "Description:\n\nPull\nRequest\nDescription\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := map[string]string{
				"PullRequestTitle":       "PullRequestTitle",
				"PullRequestDescription": "Pull\nRequest\nDescription\n",
			}
			got, got1 := expandDefaultMergeMessage(tt.template, vars)
			assert.Equalf(t, "Merge PullRequestTitle", got, "expandDefaultMergeMessage(%v, %v)", tt.template, vars)
			assert.Equalf(t, tt.wantBody, got1, "expandDefaultMergeMessage(%v, %v)", tt.template, vars)
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
	for _, tt := range []struct {
		name string
		want string
	}{
		{"NoDefault", ""},
		{"WithDefault", "default template"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var files []git.FastImportFile
			if tt.want != "" {
				files = append(files, git.FastImportFile{Path: ".gitea/default_merge_message/DEFAULT_TEMPLATE.md", Content: tt.want})
			}
			files = append(files, git.FastImportFile{Path: ".gitea/default_merge_message/REBASE_TEMPLATE.md", Content: "rebase template"})
			repo, err := git.ForceFastImportWithInit(t.Context(), t.TempDir(), []git.FastImportCommit{
				{Ref: "refs/heads/master", Files: files},
			})
			require.NoError(t, err)
			gitRepo, err := git.OpenRepository(t.Context(), repo)
			require.NoError(t, err)
			defer gitRepo.Close()

			commit, err := gitRepo.GetBranchCommit(t.Context(), "master")
			require.NoError(t, err)
			tmpl, err := resolveMergeMessageTemplate(t.Context(), gitRepo, commit, "merge")
			assert.NoError(t, err)
			assert.Equal(t, tt.want, tmpl)
			tmpl, err = resolveMergeMessageTemplate(t.Context(), gitRepo, commit, "rebase")
			assert.NoError(t, err)
			assert.Equal(t, "rebase template", tmpl)
		})
	}
}

func TestMergeIntentWithMissingCommit(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, recordMergeIntent(ctx, pr, &user_model.User{ID: 2}, strings.Repeat("a", 40), false))

	merged, err := hasPullRequestCommitBeenMerged(ctx, pr)
	require.NoError(t, err)
	assert.False(t, merged)

	require.NoError(t, clearMergeIntent(ctx, pr))
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.Empty(t, pr.MergedCommitID)
	assert.Zero(t, pr.MergedUnix)
}
