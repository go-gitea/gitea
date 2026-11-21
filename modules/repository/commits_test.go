// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestPushCommits_ToAPIPayloadCommits(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pushCommits := NewPushCommits()
	pushCommits.Commits = []*PushCommit{
		{
			Sha1:           "69554a6",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "not signed commit",
		},
		{
			Sha1:           "27566bd",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "good signed commit (with not yet validated email)",
		},
		{
			Sha1:           "5099b81",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "good signed commit",
		},
	}
	pushCommits.HeadCommit = &PushCommit{Sha1: "69554a6"}

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
	payloadCommits, headCommit, err := pushCommits.ToAPIPayloadCommits(t.Context(), repo)
	assert.NoError(t, err)
	assert.Len(t, payloadCommits, 3)
	assert.NotNil(t, headCommit)

	for _, tt := range []struct {
		commit           *api.PayloadCommit
		id, message, url string
		added, modified  []string
	}{
		{payloadCommits[0], "69554a6", "not signed commit", "https://try.gitea.io/user2/repo16/commit/69554a6", []string{}, []string{"readme.md"}},
		{payloadCommits[1], "27566bd", "good signed commit (with not yet validated email)", "https://try.gitea.io/user2/repo16/commit/27566bd", []string{}, []string{"readme.md"}},
		{payloadCommits[2], "5099b81", "good signed commit", "https://try.gitea.io/user2/repo16/commit/5099b81", []string{"readme.md"}, []string{}},
		{headCommit, "69554a6", "not signed commit", "https://try.gitea.io/user2/repo16/commit/69554a6", []string{}, []string{"readme.md"}},
	} {
		assert.Equal(t, tt.id, tt.commit.ID)
		assert.Equal(t, tt.message, tt.commit.Message)
		assert.Equal(t, tt.url, tt.commit.URL)
		assert.Equal(t, "User2", tt.commit.Committer.Name)
		assert.Equal(t, "user2", tt.commit.Committer.UserName)
		assert.Equal(t, "User2", tt.commit.Author.Name)
		assert.Equal(t, "user2", tt.commit.Author.UserName)
		assert.Equal(t, tt.added, tt.commit.Added)
		assert.Equal(t, []string{}, tt.commit.Removed)
		assert.Equal(t, tt.modified, tt.commit.Modified)
	}
}

func TestCommitToPushCommit(t *testing.T) {
	now := time.Now()
	sig := &git.Signature{
		Email: "example@example.com",
		Name:  "John Doe",
		When:  now,
	}
	const hexString = "0123456789abcdef0123456789abcdef01234567"
	sha1, err := git.NewIDFromString(hexString)
	assert.NoError(t, err)
	pushCommit := CommitToPushCommit(&git.Commit{
		ID:            sha1,
		Author:        sig,
		Committer:     sig,
		CommitMessage: git.CommitMessage{MessageRaw: "Commit Message"},
	})
	assert.Equal(t, hexString, pushCommit.Sha1)
	assert.Equal(t, "Commit Message", pushCommit.Message)
	assert.Equal(t, "example@example.com", pushCommit.AuthorEmail)
	assert.Equal(t, "John Doe", pushCommit.AuthorName)
	assert.Equal(t, "example@example.com", pushCommit.CommitterEmail)
	assert.Equal(t, "John Doe", pushCommit.CommitterName)
	assert.Equal(t, now, pushCommit.Timestamp)
}

func TestListToPushCommits(t *testing.T) {
	now := time.Now()
	sig := &git.Signature{
		Email: "example@example.com",
		Name:  "John Doe",
		When:  now,
	}

	const hexString1 = "0123456789abcdef0123456789abcdef01234567"
	hash1, err := git.NewIDFromString(hexString1)
	assert.NoError(t, err)
	const hexString2 = "fedcba9876543210fedcba9876543210fedcba98"
	hash2, err := git.NewIDFromString(hexString2)
	assert.NoError(t, err)

	l := []*git.Commit{
		{
			ID:            hash1,
			Author:        sig,
			Committer:     sig,
			CommitMessage: git.CommitMessage{MessageRaw: "Message1"},
		},
		{
			ID:            hash2,
			Author:        sig,
			Committer:     sig,
			CommitMessage: git.CommitMessage{MessageRaw: "Message2"},
		},
	}

	pushCommits := GitToPushCommits(l)
	if assert.Len(t, pushCommits.Commits, 2) {
		assert.Equal(t, "Message1", pushCommits.Commits[0].Message)
		assert.Equal(t, hexString1, pushCommits.Commits[0].Sha1)
		assert.Equal(t, "example@example.com", pushCommits.Commits[0].AuthorEmail)
		assert.Equal(t, now, pushCommits.Commits[0].Timestamp)

		assert.Equal(t, "Message2", pushCommits.Commits[1].Message)
		assert.Equal(t, hexString2, pushCommits.Commits[1].Sha1)
		assert.Equal(t, "example@example.com", pushCommits.Commits[1].AuthorEmail)
		assert.Equal(t, now, pushCommits.Commits[1].Timestamp)
	}
}
