// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"
	"testing"
	"unicode/utf8"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	git_service "code.gitea.io/gitea/services/git"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestAttachCommentsToLines(t *testing.T) {
	section := &gitdiff.DiffSection{
		Lines: []*gitdiff.DiffLine{
			{LeftIdx: 5, RightIdx: 10},
			{LeftIdx: 6, RightIdx: 11},
		},
	}

	lineComments := map[int64][]*issues_model.Comment{
		-5: {{ID: 100, CreatedUnix: 1000}},                               // left side comment
		10: {{ID: 200, CreatedUnix: 2000}},                               // right side comment
		11: {{ID: 300, CreatedUnix: 1500}, {ID: 301, CreatedUnix: 2500}}, // multiple comments
	}

	attachCommentsToLines(section, lineComments)

	// First line should have left and right comments
	assert.Len(t, section.Lines[0].Comments, 2)
	assert.Equal(t, int64(100), section.Lines[0].Comments[0].ID)
	assert.Equal(t, int64(200), section.Lines[0].Comments[1].ID)

	// Second line should have two comments, sorted by creation time
	assert.Len(t, section.Lines[1].Comments, 2)
	assert.Equal(t, int64(300), section.Lines[1].Comments[0].ID)
	assert.Equal(t, int64(301), section.Lines[1].Comments[1].ID)
}

func TestNewPullRequestTitleContent(t *testing.T) {
	ci := &git_service.CompareInfo{HeadRef: "refs/heads/head-branch"}

	mockCommit := func(msg string) *git_model.SignCommitWithStatuses {
		return &git_model.SignCommitWithStatuses{
			SignCommit: &asymkey_model.SignCommit{
				UserCommit: &user_model.UserCommit{
					Commit: &git.Commit{
						CommitMessage: msg,
					},
				},
			},
		}
	}

	title, content := prepareNewPullRequestTitleContent(ci, nil)
	assert.Equal(t, "head-branch", title)
	assert.Empty(t, content)

	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("title-only")})
	assert.Equal(t, "title-only", title)
	assert.Empty(t, content)

	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("title-" + strings.Repeat("a", 255))})
	assert.Equal(t, "title-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa…", title)
	assert.Equal(t, "…aaaaaaaaa\n", content)

	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("title\nbody")})
	assert.Equal(t, "title", title)
	assert.Equal(t, "body", content)

	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{mockCommit("a\xf0\xf0\xf0\nb\xf0\xf0\xf0")})
	assert.Equal(t, "a?", title) // FIXME: GIT-COMMIT-MESSAGE-ENCODING: "title" doesn't use the same charset converting logic as "content"
	assert.Equal(t, "b"+string(utf8.RuneError)+string(utf8.RuneError), content)

	title, content = prepareNewPullRequestTitleContent(ci, []*git_model.SignCommitWithStatuses{
		// ordered from newest to oldest
		mockCommit("title2\nbody2"),
		mockCommit("title1\nbody1"),
	})
	assert.Equal(t, "title1", title)
	assert.Empty(t, content)
}
