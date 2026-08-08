// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"path/filepath"
	"testing"

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
		repo, err := git.ForceFastImportWithInit(t.Context(), filepath.Join(t.TempDir(), "test-repo"), []git.FastImportCommit{
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
		repo, err := git.ForceFastImportWithInit(t.Context(), filepath.Join(t.TempDir(), "test-repo"), []git.FastImportCommit{
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
