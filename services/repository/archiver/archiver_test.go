// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package archiver

import (
	"bytes"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"
	"gitea.dev/services/contexttest"

	_ "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestArchive_Basic(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx, _ := contexttest.MockContext(t, "user27/repo49")
	firstCommit, secondCommit := "51f84af23134", "aacbdfe9e1c4"

	contexttest.LoadRepo(t, ctx, 49)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	bogusReq, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, firstCommit+".zip", nil)
	assert.NoError(t, err)
	assert.NotNil(t, bogusReq)
	assert.Equal(t, firstCommit+".zip", bogusReq.GetArchiveName())

	// Check a series of bogus requests.
	// Step 1, valid commit with a bad extension.
	bogusReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, firstCommit+".unknown", nil)
	assert.Error(t, err)
	assert.Nil(t, bogusReq)

	// Step 2, missing commit.
	bogusReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "dbffff.zip", nil)
	assert.Error(t, err)
	assert.Nil(t, bogusReq)

	// Step 3, doesn't look like branch/tag/commit.
	bogusReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "db.zip", nil)
	assert.Error(t, err)
	assert.Nil(t, bogusReq)

	bogusReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "master.zip", nil)
	assert.NoError(t, err)
	assert.NotNil(t, bogusReq)
	assert.Equal(t, "master.zip", bogusReq.GetArchiveName())

	bogusReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "test/archive.zip", nil)
	assert.NoError(t, err)
	assert.NotNil(t, bogusReq)
	assert.Equal(t, "test-archive.zip", bogusReq.GetArchiveName())

	// Now two valid requests, firstCommit with valid extensions.
	zipReq, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, firstCommit+".zip", nil)
	assert.NoError(t, err)
	assert.NotNil(t, zipReq)

	tgzReq, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, firstCommit+".tar.gz", nil)
	assert.NoError(t, err)
	assert.NotNil(t, tgzReq)

	secondReq, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, secondCommit+".bundle", nil)
	assert.NoError(t, err)
	assert.NotNil(t, secondReq)

	for _, request := range []*ArchiveRequest{zipReq, tgzReq, secondReq} {
		var archive bytes.Buffer
		require.NoError(t, request.Stream(ctx, &archive))
		assert.Positive(t, archive.Len())
	}

	// Same commit, different compression formats should have different names.
	// Ideally, the extension would match what we originally requested.
	assert.NotEqual(t, zipReq.GetArchiveName(), tgzReq.GetArchiveName())
	assert.NotEqual(t, zipReq.GetArchiveName(), secondReq.GetArchiveName())

	t.Run("BadPath", func(t *testing.T) {
		badRequest, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, firstCommit+".tar.gz", []string{"not-a-path"})
		require.NoError(t, err)
		err = ServeRepoArchive(ctx.Base, badRequest)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
		assert.ErrorContains(t, err, "path doesn't exist or is invalid")
	})
}
