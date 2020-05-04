// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package archiver

import (
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/com"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestArchive_Basic(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	ctx := test.MockContext(t, "user27/repo49")
	firstCommit, secondCommit := "51f84af23134", "aacbdfe9e1c4"

	bogusReq := DeriveRequestFrom(ctx, firstCommit+".zip")
	assert.Nil(t, bogusReq)

	test.LoadRepo(t, ctx, 49)
	bogusReq = DeriveRequestFrom(ctx, firstCommit+".zip")
	assert.Nil(t, bogusReq)

	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	// Check a series of bogus requests.
	// Step 1, valid commit with a bad extension.
	bogusReq = DeriveRequestFrom(ctx, firstCommit+".dilbert")
	assert.Nil(t, bogusReq)

	// Step 2, missing commit.
	bogusReq = DeriveRequestFrom(ctx, "dbffff.zip")
	assert.Nil(t, bogusReq)

	// Step 3, doesn't look like branch/tag/commit.
	bogusReq = DeriveRequestFrom(ctx, "db.zip")
	assert.Nil(t, bogusReq)

	// Now two valid requests, firstCommit with valid extensions.
	zipReq := DeriveRequestFrom(ctx, firstCommit+".zip")
	assert.NotNil(t, zipReq)

	tgzReq := DeriveRequestFrom(ctx, firstCommit+".tar.gz")
	assert.NotNil(t, tgzReq)

	secondReq := DeriveRequestFrom(ctx, secondCommit+".zip")
	assert.NotNil(t, secondReq)

	ArchiveRepository(zipReq)
	ArchiveRepository(tgzReq)
	ArchiveRepository(secondReq)

	// Wait for those requests to complete, time out after 8 seconds.
	timeout := time.Now().Add(8 * time.Second)
	for {
		if zipReq.IsComplete() && tgzReq.IsComplete() && secondReq.IsComplete() {
			break
		} else if time.Now().After(timeout) {
			break
		}
	}

	assert.True(t, zipReq.IsComplete())
	assert.True(t, tgzReq.IsComplete())
	assert.True(t, secondReq.IsComplete())
	assert.True(t, com.IsExist(zipReq.GetArchivePath()))
	assert.True(t, com.IsExist(tgzReq.GetArchivePath()))
	assert.True(t, com.IsExist(secondReq.GetArchivePath()))

	// The queue should also be drained, if all requests have completed.
	assert.Equal(t, len(archiveInProgress), 0)

	zipReq2 := DeriveRequestFrom(ctx, firstCommit+".zip")
	// After completion, zipReq should have dropped out of the queue.  Make sure
	// we didn't get it handed back to us, but they should otherwise be
	// equivalent requests.
	assert.Equal(t, zipReq, zipReq2)
	assert.False(t, zipReq == zipReq2)

	// Make sure we can submit this follow-up request with no side-effects, to
	// the extent that we can.
	ArchiveRepository(zipReq2)
	assert.Equal(t, zipReq, zipReq2)
	assert.Equal(t, len(archiveInProgress), 0)

	// Same commit, different compression formats should have different names.
	// Ideally, the extension would match what we originally requested.
	assert.NotEqual(t, zipReq.GetArchiveName(), tgzReq.GetArchiveName())
	assert.NotEqual(t, zipReq.GetArchiveName(), secondReq.GetArchiveName())
}
