// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package archiver

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/com"
)

var queueMutex sync.Mutex

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))

	archiveQueueMutex = &queueMutex
	archiveQueueStartCond = sync.NewCond(&queueMutex)
	archiveQueueReleaseCond = sync.NewCond(&queueMutex)
}

func allComplete(inFlight []*ArchiveRequest) bool {
	for _, req := range inFlight {
		if !req.IsComplete() {
			return false
		}
	}

	return true
}

func waitForCount(t *testing.T, num int) {
	var numQueued int

	// Wait for 3 seconds to hit the queue.
	timeout := time.Now().Add(3 * time.Second)
	for {
		numQueued = len(archiveInProgress)
		if numQueued == num || time.Now().After(timeout) {
			break
		}
	}

	assert.Equal(t, num, len(archiveInProgress))
}

func releaseOneEntry(t *testing.T, inFlight []*ArchiveRequest) {
	var nowQueued, numQueued int

	numQueued = len(archiveInProgress)

	// Release one, then wait up to 3 seconds for it to complete.
	queueMutex.Lock()
	archiveQueueReleaseCond.Signal()
	queueMutex.Unlock()
	timeout := time.Now().Add(3 * time.Second)
	for {
		nowQueued = len(archiveInProgress)
		if nowQueued != numQueued || time.Now().After(timeout) {
			break
		}
	}

	// Make sure we didn't just timeout.
	assert.NotEqual(t, nowQueued, numQueued)

	// Also make sure that we released only one.
	assert.Equal(t, nowQueued, numQueued-1)
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

	inFlight := make([]*ArchiveRequest, 3)
	inFlight[0] = zipReq
	inFlight[1] = tgzReq
	inFlight[2] = secondReq

	ArchiveRepository(zipReq)
	waitForCount(t, 1)
	ArchiveRepository(tgzReq)
	waitForCount(t, 2)
	ArchiveRepository(secondReq)
	waitForCount(t, 3)

	// Make sure sending an unprocessed request through doesn't affect the queue
	// count.
	ArchiveRepository(zipReq)

	// Sleep two seconds to make sure the queue doesn't change.
	twoSeconds, _ := time.ParseDuration("2s")
	time.Sleep(twoSeconds)
	assert.Equal(t, 3, len(archiveInProgress))

	// Release them all, they'll then stall at the archiveQueueReleaseCond while
	// we examine the queue state.
	queueMutex.Lock()
	archiveQueueStartCond.Broadcast()
	queueMutex.Unlock()

	// 8 second timeout for them all to complete.
	timeout := time.Now().Add(8 * time.Second)
	for {
		if allComplete(inFlight) {
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

	// Queues should not have drained yet, because we haven't released them.
	// Do so now.
	assert.Equal(t, len(archiveInProgress), 3)

	zipReq2 := DeriveRequestFrom(ctx, firstCommit+".zip")
	// After completion, zipReq should have dropped out of the queue.  Make sure
	// we didn't get it handed back to us, but they should otherwise be
	// equivalent requests.
	assert.Equal(t, zipReq, zipReq2)
	assert.False(t, zipReq == zipReq2)

	// We still have the other three stalled at completion, waiting to remove
	// from archiveInProgress.  Try to submit this new one before its
	// predecessor has cleared out of the queue.
	ArchiveRepository(zipReq2)

	// Make sure we didn't enqueue anything from this new one, and that the
	// queue hasn't changed.
	assert.Equal(t, len(archiveInProgress), 3)

	for _, req := range archiveInProgress {
		assert.False(t, req == zipReq2)
	}

	// Make sure the queue drains properly
	releaseOneEntry(t, inFlight)
	assert.Equal(t, len(archiveInProgress), 2)
	releaseOneEntry(t, inFlight)
	assert.Equal(t, len(archiveInProgress), 1)
	releaseOneEntry(t, inFlight)
	assert.Equal(t, len(archiveInProgress), 0)

	// Same commit, different compression formats should have different names.
	// Ideally, the extension would match what we originally requested.
	assert.NotEqual(t, zipReq.GetArchiveName(), tgzReq.GetArchiveName())
	assert.NotEqual(t, zipReq.GetArchiveName(), secondReq.GetArchiveName())
}
