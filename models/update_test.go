// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"container/list"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

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

	l := list.New()
	l.PushBack(&git.Commit{
		ID:            hash1,
		Author:        sig,
		Committer:     sig,
		CommitMessage: "Message1",
	})
	l.PushBack(&git.Commit{
		ID:            hash2,
		Author:        sig,
		Committer:     sig,
		CommitMessage: "Message2",
	})

	pushCommits := ListToPushCommits(l)
	assert.Equal(t, 2, pushCommits.Len)
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
