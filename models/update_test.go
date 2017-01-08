// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"container/list"
	"testing"
	"time"

	"code.gitea.io/git"

	"github.com/stretchr/testify/assert"
)

func TestAddUpdateTask(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	task := &UpdateTask{
		UUID:        "uuid4",
		RefName:     "refName4",
		OldCommitID: "oldCommitId4",
		NewCommitID: "newCommitId4",
	}
	assert.NoError(t, AddUpdateTask(task))

	sess := x.NewSession()
	defer sess.Close()
	has, err := sess.Get(task)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, "uuid4", task.UUID)
}

func TestGetUpdateTaskByUUID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	task, err := GetUpdateTaskByUUID("uuid1")
	assert.NoError(t, err)
	assert.Equal(t, "uuid1", task.UUID)
	assert.Equal(t, "refName1", task.RefName)
	assert.Equal(t, "oldCommitId1", task.OldCommitID)
	assert.Equal(t, "newCommitId1", task.NewCommitID)

	_, err = GetUpdateTaskByUUID("invalid")
	assert.Error(t, err)
	assert.True(t, IsErrUpdateTaskNotExist(err))
}

func TestDeleteUpdateTaskByUUID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, DeleteUpdateTaskByUUID("uuid1"))
	sess := x.NewSession()
	defer sess.Close()
	has, err := sess.Get(&UpdateTask{UUID: "uuid1"})
	assert.NoError(t, err)
	assert.False(t, has)

	assert.NoError(t, DeleteUpdateTaskByUUID("invalid"))
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
	assert.Equal(t, 2, len(pushCommits.Commits))

	assert.Equal(t, "Message1", pushCommits.Commits[0].Message)
	assert.Equal(t, hexString1, pushCommits.Commits[0].Sha1)
	assert.Equal(t, "example@example.com", pushCommits.Commits[0].AuthorEmail)
	assert.Equal(t, now, pushCommits.Commits[0].Timestamp)

	assert.Equal(t, "Message2", pushCommits.Commits[1].Message)
	assert.Equal(t, hexString2, pushCommits.Commits[1].Sha1)
	assert.Equal(t, "example@example.com", pushCommits.Commits[1].AuthorEmail)
	assert.Equal(t, now, pushCommits.Commits[1].Timestamp)
}

// TODO TestPushUpdate
