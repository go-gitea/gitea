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
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
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

func TestBuildSquashMergeCommitMessages(t *testing.T) {
	trailers := []string{"Signed-off-by: a <a@example.com>", "Co-authored-by: b <b@example.com>"}
	assert.Equal(t, "body\n\nSigned-off-by: a <a@example.com>\nCo-authored-by: b <b@example.com>", buildSquashMergeCommitMessages("body", trailers, false))
	assert.Equal(t, "body\n\n---------\n\nSigned-off-by: a <a@example.com>\nCo-authored-by: b <b@example.com>", buildSquashMergeCommitMessages("body", trailers, true))
	assert.Equal(t, "body\n\nRefs: #1\nsigned-off-by:  a <a@example.com>\nCo-authored-by: b <b@example.com>", buildSquashMergeCommitMessages("body\n\nRefs: #1\nsigned-off-by:  a <a@example.com>", trailers, true))
}

func TestCollectSquashMergeCommitTrailers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	newCommit := func(authorName, msg string) *git.Commit {
		return &git.Commit{Author: &git.Signature{Name: authorName, Email: authorName + "@example.com"}, CommitMessage: git.CommitMessage{MessageRaw: msg}}
	}

	trailers, err := collectSquashMergeCommitTrailers(t.Context(), poster, []*git.Commit{
		newCommit("whiskey", "third\n\nCo-authored-by: renamed <ZULU@example.com>"),
		newCommit("user2", "second\n\nSigned-off-by: one <one@example.com>\nSigned-off-by: two <two@example.com>"),
		newCommit("zulu", "first\n\nCo-authored-by: yankee <yankee@example.com>\nSigned-off-by: two <two@example.com>"),
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"Signed-off-by: two <two@example.com>",
		"Signed-off-by: one <one@example.com>",
		"Co-authored-by: zulu <zulu@example.com>",
		"Co-authored-by: yankee <yankee@example.com>",
		"Co-authored-by: whiskey <whiskey@example.com>",
	}, trailers)
}
