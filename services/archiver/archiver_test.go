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
}

func waitForCount(t *testing.T, num int) {
	var numQueued int

	// Wait for up to 10 seconds for the queue to be impacted.
	timeout := time.Now().Add(10 * time.Second)
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

	// Release one, then wait up to 10 seconds for it to complete.
	queueMutex.Lock()
	archiveQueueReleaseCond.Signal()
	queueMutex.Unlock()
	timeout := time.Now().Add(10 * time.Second)
	for {
		nowQueued = len(archiveInProgress)
		if nowQueued != numQueued || time.Now().After(timeout) {
			break
		}
	}

	// Make sure we didn't just timeout.
	assert.NotEqual(t, numQueued, nowQueued)

	// Also make sure that we released only one.
	assert.Equal(t, numQueued-1, nowQueued)
}

func TestArchive_Basic(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	archiveQueueMutex = &queueMutex
	archiveQueueStartCond = sync.NewCond(&queueMutex)
	archiveQueueReleaseCond = sync.NewCond(&queueMutex)
	defer func() {
		archiveQueueMutex = nil
		archiveQueueStartCond = nil
		archiveQueueReleaseCond = nil
	}()

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
	time.Sleep(2 * time.Second)
	assert.Equal(t, 3, len(archiveInProgress))

	// Release them all, they'll then stall at the archiveQueueReleaseCond while
	// we examine the queue state.
	queueMutex.Lock()
	archiveQueueStartCond.Broadcast()
	queueMutex.Unlock()

	// Iterate through all of the in-flight requests and wait for their
	// completion.
	for _, req := range inFlight {
		req.WaitForCompletion(ctx)
	}

	for _, req := range inFlight {
		assert.True(t, req.IsComplete())
		assert.True(t, com.IsExist(req.GetArchivePath()))
	}

	arbitraryReq := inFlight[0]
	// Reopen the channel so we don't double-close, mark it incomplete.  We're
	// going to run it back through the archiver, and it should get marked
	// complete again.
	arbitraryReq.cchan = make(chan struct{})
	arbitraryReq.archiveComplete = false
	doArchive(arbitraryReq)
	assert.True(t, arbitraryReq.IsComplete())

	// Queues should not have drained yet, because we haven't released them.
	// Do so now.
	assert.Equal(t, 3, len(archiveInProgress))

	zipReq2 := DeriveRequestFrom(ctx, firstCommit+".zip")
	// This zipReq should match what's sitting in the queue, as we haven't
	// let it release yet.  From the consumer's point of view, this looks like
	// a long-running archive task.
	assert.Equal(t, zipReq, zipReq2)

	// We still have the other three stalled at completion, waiting to remove
	// from archiveInProgress.  Try to submit this new one before its
	// predecessor has cleared out of the queue.
	ArchiveRepository(zipReq2)

	// Make sure the queue hasn't grown any.
	assert.Equal(t, 3, len(archiveInProgress))

	// Make sure the queue drains properly
	releaseOneEntry(t, inFlight)
	assert.Equal(t, 2, len(archiveInProgress))
	releaseOneEntry(t, inFlight)
	assert.Equal(t, 1, len(archiveInProgress))
	releaseOneEntry(t, inFlight)
	assert.Equal(t, 0, len(archiveInProgress))

	// Now we'll submit a request and TimedWaitForCompletion twice, before and
	// after we release it.  We should trigger both the timeout and non-timeout
	// cases.
	var completed, timedout bool
	timedReq := DeriveRequestFrom(ctx, secondCommit+".tar.gz")
	assert.NotNil(t, timedReq)
	ArchiveRepository(timedReq)

	// Guaranteed to timeout; we haven't signalled the request to start..
	completed, timedout = timedReq.TimedWaitForCompletion(ctx, 2*time.Second)
	assert.Equal(t, false, completed)
	assert.Equal(t, true, timedout)

	queueMutex.Lock()
	archiveQueueStartCond.Broadcast()
	queueMutex.Unlock()

	// Shouldn't timeout, we've now signalled it and it's a small request.
	completed, timedout = timedReq.TimedWaitForCompletion(ctx, 15*time.Second)
	assert.Equal(t, true, completed)
	assert.Equal(t, false, timedout)

	zipReq2 = DeriveRequestFrom(ctx, firstCommit+".zip")
	// Now, we're guaranteed to have released the original zipReq from the queue.
	// Ensure that we don't get handed back the released entry somehow, but they
	// should remain functionally equivalent in all fields.  The exception here
	// is zipReq.cchan, which will be non-nil because it's a completed request.
	// It's fine to go ahead and set it to nil now.
	zipReq.cchan = nil
	assert.Equal(t, zipReq, zipReq2)
	assert.False(t, zipReq == zipReq2)

	// Same commit, different compression formats should have different names.
	// Ideally, the extension would match what we originally requested.
	assert.NotEqual(t, zipReq.GetArchiveName(), tgzReq.GetArchiveName())
	assert.NotEqual(t, zipReq.GetArchiveName(), secondReq.GetArchiveName())
}
