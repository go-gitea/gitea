// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

func PrepareOldRepository(t *testing.T) (*xorm.Engine, func()) {
	type Repository struct { // old struct
		ID int64 `xorm:"pk autoincr"`
	}

	type CommitStatus struct {
		ID          int64
		ContextHash string
	}

	type RepoArchiver struct {
		ID       int64
		RepoID   int64
		Type     int
		CommitID string
	}

	type ReviewState struct {
		ID        int64
		CommitSHA string
		UserID    int64
		PullID    int64
	}

	type Comment struct {
		ID        int64
		CommitSHA string
	}

	type PullRequest struct {
		ID             int64
		CommitSHA      string
		MergeBase      string
		MergedCommitID string
	}

	type Release struct {
		ID   int64
		Sha1 string
	}

	type RepoIndexerStatus struct {
		ID        int64
		CommitSHA string
	}

	type Review struct {
		ID       int64
		CommitID string
	}

	// Prepare and load the testing database
	return base.PrepareTestEnv(t, 0,
		new(Repository),
		new(CommitStatus),
		new(RepoArchiver),
		new(ReviewState),
		new(Review),
		new(Comment),
		new(PullRequest),
		new(Release),
		new(RepoIndexerStatus),
	)
}

func Test_RepositoryFormat(t *testing.T) {
	x, deferable := PrepareOldRepository(t)
	defer deferable()

	assert.NoError(t, AdjustDBForSha256(x))

	type Repository struct {
		ID               int64  `xorm:"pk autoincr"`
		ObjectFormatName string `xorg:"not null default('sha1')"`
	}

	repo := new(Repository)

	// check we have some records to migrate
	count, err := x.Count(new(Repository))
	assert.NoError(t, err)
	assert.EqualValues(t, 4, count)

	repo.ObjectFormatName = "sha256"
	_, err = x.Insert(repo)
	assert.NoError(t, err)
	id := repo.ID

	count, err = x.Count(new(Repository))
	assert.NoError(t, err)
	assert.EqualValues(t, 5, count)

	repo = new(Repository)
	ok, err := x.ID(2).Get(repo)
	assert.NoError(t, err)
	assert.EqualValues(t, true, ok)
	assert.EqualValues(t, "sha1", repo.ObjectFormatName)

	repo = new(Repository)
	ok, err = x.ID(id).Get(repo)
	assert.NoError(t, err)
	assert.EqualValues(t, true, ok)
	assert.EqualValues(t, "sha256", repo.ObjectFormatName)
}
