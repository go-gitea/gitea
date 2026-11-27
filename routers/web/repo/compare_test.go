// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	git_model "gitea.dev/models/git"
	"gitea.dev/models/gituser"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	git_service "gitea.dev/services/git"

	"github.com/stretchr/testify/assert"
)

func TestNewPullRequestTitleContent(t *testing.T) {
	ci := &git_service.CompareInfo{HeadRef: "refs/heads/head-branch"}

	mockCommit := func(msg string) *git_model.SignCommitWithStatuses {
		return &git_model.SignCommitWithStatuses{
			SignCommit: &asymkey_model.SignCommit{
				UserCommit: &gituser.UserCommit{
					GitCommit: &git.Commit{
						CommitMessage: git.CommitMessage{MessageRaw: msg},
					},
				},
			},
		}
	}

	// no commit
	title, content := prepareNewPullRequestTitleContent(ci, nil, setting.RepoPRTitleSourceAuto)
	assert.Equal(t, "Head branch", title)
	assert.Empty(t, content)

	title, content = prepareNewPullRequestTitleContent(ci, nil, setting.RepoPRTitleSourceFirstCommit)
	assert.Equal(t, "Head branch", title)
	assert.Empty(t, content)

	title, content = prepareNewPullRequestTitleContent(ci, nil, setting.RepoPRTitleSourceBranchName)
	assert.Equal(t, "head-branch", title)
	assert.Empty(t, content)

	// single commit
	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("single-commit-title\nbody")}, setting.RepoPRTitleSourceAuto)
	assert.Equal(t, "single-commit-title", title)
	assert.Equal(t, "body", content)

	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("single-commit-title\nbody")}, setting.RepoPRTitleSourceFirstCommit)
	assert.Equal(t, "single-commit-title", title)
	assert.Equal(t, "body", content)

	// multiple commits
	commits := []*git_model.SignCommitWithStatuses{
		// ordered from newest to oldest
		mockCommit("title2\nbody2"),
		mockCommit("title1\nbody1"),
	}
	title, content = prepareNewPullRequestTitleContent(ci, commits, setting.RepoPRTitleSourceAuto)
	assert.Equal(t, "Head branch", title)
	assert.Empty(t, content)

	title, content = prepareNewPullRequestTitleContent(ci, commits, setting.RepoPRTitleSourceFirstCommit)
	assert.Equal(t, "title1", title)
	assert.Empty(t, content)

	// title string handling
	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("title-" + strings.Repeat("a", 255))}, setting.RepoPRTitleSourceFirstCommit)
	assert.Equal(t, "title-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa…", title)
	assert.Equal(t, "…aaaaaaaaa\n", content)

	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("title \xf0\nbody \xf0")}, setting.RepoPRTitleSourceFirstCommit)
	assert.Equal(t, "title ð", title)
	assert.Equal(t, "body ð", content)
}

func TestAutoTitleFromBranchName(t *testing.T) {
	cases := []struct {
		branch string
		want   string
	}{
		{"fix/the-bug", "Fix/the bug"},
		{"Already-Capitalized", "Already capitalized"},
		{"ALL-CAPS-BRANCH", "All caps branch"},
		{"FixHTMLBug", "Fix html bug"},
		{"MixedCase-Name", "Mixed case name"},
		{"fooBar-baz", "Foo bar baz"},
		{"foo/BAR", "Foo/bar"},
		{"_leading-underscore", "Leading underscore"},
		{"CamelCase", "Camel case"},
		{"foo--double-dash", "Foo double dash"},
		{"123-fix", "123 fix"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, autoTitleFromBranchName(c.branch), "branch: %q", c.branch)
	}
}
