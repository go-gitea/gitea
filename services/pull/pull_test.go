// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
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

func TestBuildSquashMergeCommitMessages(t *testing.T) {
	cases := []struct {
		msg       string
		coAuthors []string
		expected  string
	}{
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
