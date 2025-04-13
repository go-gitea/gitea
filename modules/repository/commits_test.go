// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"

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
	payloadCommits, headCommit, err := pushCommits.ToAPIPayloadCommits(git.DefaultContext, repo)
	assert.NoError(t, err)
	assert.Len(t, payloadCommits, 3)
	assert.NotNil(t, headCommit)

	assert.Equal(t, "69554a6", payloadCommits[0].ID)
	assert.Equal(t, "not signed commit", payloadCommits[0].Message)
	assert.Equal(t, "https://try.gitea.io/user2/repo16/commit/69554a6", payloadCommits[0].URL)
	assert.Equal(t, "User2", payloadCommits[0].Committer.Name)
	assert.Equal(t, "user2", payloadCommits[0].Committer.UserName)
	assert.Equal(t, "User2", payloadCommits[0].Author.Name)
	assert.Equal(t, "user2", payloadCommits[0].Author.UserName)
	assert.Equal(t, []string{}, payloadCommits[0].Added)
	assert.Equal(t, []string{}, payloadCommits[0].Removed)
	assert.Equal(t, []string{"readme.md"}, payloadCommits[0].Modified)

	assert.Equal(t, "27566bd", payloadCommits[1].ID)
	assert.Equal(t, "good signed commit (with not yet validated email)", payloadCommits[1].Message)
	assert.Equal(t, "https://try.gitea.io/user2/repo16/commit/27566bd", payloadCommits[1].URL)
	assert.Equal(t, "User2", payloadCommits[1].Committer.Name)
	assert.Equal(t, "user2", payloadCommits[1].Committer.UserName)
	assert.Equal(t, "User2", payloadCommits[1].Author.Name)
	assert.Equal(t, "user2", payloadCommits[1].Author.UserName)
	assert.Equal(t, []string{}, payloadCommits[1].Added)
	assert.Equal(t, []string{}, payloadCommits[1].Removed)
	assert.Equal(t, []string{"readme.md"}, payloadCommits[1].Modified)

	assert.Equal(t, "5099b81", payloadCommits[2].ID)
	assert.Equal(t, "good signed commit", payloadCommits[2].Message)
	assert.Equal(t, "https://try.gitea.io/user2/repo16/commit/5099b81", payloadCommits[2].URL)
	assert.Equal(t, "User2", payloadCommits[2].Committer.Name)
	assert.Equal(t, "user2", payloadCommits[2].Committer.UserName)
	assert.Equal(t, "User2", payloadCommits[2].Author.Name)
	assert.Equal(t, "user2", payloadCommits[2].Author.UserName)
	assert.Equal(t, []string{"readme.md"}, payloadCommits[2].Added)
	assert.Equal(t, []string{}, payloadCommits[2].Removed)
	assert.Equal(t, []string{}, payloadCommits[2].Modified)

	assert.Equal(t, "69554a6", headCommit.ID)
	assert.Equal(t, "not signed commit", headCommit.Message)
	assert.Equal(t, "https://try.gitea.io/user2/repo16/commit/69554a6", headCommit.URL)
	assert.Equal(t, "User2", headCommit.Committer.Name)
	assert.Equal(t, "user2", headCommit.Committer.UserName)
	assert.Equal(t, "User2", headCommit.Author.Name)
	assert.Equal(t, "user2", headCommit.Author.UserName)
	assert.Equal(t, []string{}, headCommit.Added)
	assert.Equal(t, []string{}, headCommit.Removed)
	assert.Equal(t, []string{"readme.md"}, headCommit.Modified)
}

func TestPushCommits_AvatarLink(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pushCommits := NewPushCommits()
	pushCommits.Commits = []*PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "message1",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "message2",
		},
	}

	assert.Equal(t,
		"/avatars/ab53a2911ddf9b4817ac01ddcd3d975f?size="+strconv.Itoa(28*setting.Avatar.RenderedSizeFactor),
		pushCommits.AvatarLink(db.DefaultContext, "user2@example.com"))

	assert.Equal(t,
		"/assets/img/avatar_default.png",
		pushCommits.AvatarLink(db.DefaultContext, "nonexistent@example.com"))
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
		CommitMessage: "Commit Message",
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
			CommitMessage: "Message1",
		},
		{
			ID:            hash2,
			Author:        sig,
			Committer:     sig,
			CommitMessage: "Message2",
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

// TODO TestPushUpdate
