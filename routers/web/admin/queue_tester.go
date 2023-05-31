// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"runtime/pprof"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

var testQueueOnce sync.Once

// initTestQueueOnce initializes the test queue for dev mode
// the test queue will also be shown in the queue list
// developers could see the queue length / worker number / items number on the admin page and try to remove the items
func initTestQueueOnce() {
	testQueueOnce.Do(func() {
		ctx, _, finished := process.GetManager().AddTypedContext(graceful.GetManager().ShutdownContext(), "TestQueue", process.SystemProcessType, false)
		qs := setting.QueueSettings{
			Name:        "test-queue",
			Type:        "channel",
			Length:      20,
			BatchLength: 2,
			MaxWorkers:  3,
		}
		testQueue, err := queue.NewWorkerPoolQueueWithContext(ctx, "test-queue", qs, func(t ...int64) (unhandled []int64) {
			for range t {
				select {
				case <-graceful.GetManager().ShutdownContext().Done():
				case <-time.After(5 * time.Second):
				}
			}
			return nil
		}, true)
		if err != nil {
			log.Error("unable to create test queue: %v", err)
			return
		}

		queue.GetManager().AddManagedQueue(testQueue)
		testQueue.SetWorkerMaxNumber(5)
		go graceful.GetManager().RunWithCancel(testQueue)
		go func() {
			pprof.SetGoroutineLabels(ctx)
			defer finished()

			cnt := int64(0)
			adding := true
			for {
				select {
				case <-ctx.Done():
				case <-time.After(500 * time.Millisecond):
					if adding {
						if testQueue.GetQueueItemNumber() == qs.Length {
							adding = false
						}
					} else {
						if testQueue.GetQueueItemNumber() == 0 {
							adding = true
						}
					}
					if adding {
						_ = testQueue.Push(cnt)
						cnt++
					}
				}
			}
		}()
	})
}
