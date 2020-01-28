// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"context"
	"net/http"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/queue"

	"gitea.com/macaron/macaron"
)

// FlushQueues flushes all the Queues
func FlushQueues(ctx *macaron.Context, opts private.FlushOptions) {
	var flushCtx context.Context
	var cancel context.CancelFunc
	start := time.Now()
	end := start
	hasTimeout := false

	baseCtx := ctx.Req.Request.Context()
	if opts.NonBlocking {
		baseCtx = graceful.GetManager().HammerContext()
	}

	if opts.Timeout > 0 {
		flushCtx, cancel = context.WithTimeout(baseCtx, opts.Timeout)
		end = start.Add(opts.Timeout)
		hasTimeout = true
	} else {
		flushCtx, cancel = context.WithCancel(baseCtx)
	}

	if opts.NonBlocking {
		go func() {
			err := doFlush(flushCtx, cancel, start, end, hasTimeout)
			if err != nil {
				log.Error("Flushing request timed-out with error: %v", err)
			}
		}()
		ctx.JSON(http.StatusAccepted, map[string]interface{}{
			"err": "Flushing",
		})
		return
	}
	err := doFlush(flushCtx, cancel, start, end, hasTimeout)
	if err != nil {
		ctx.JSON(http.StatusRequestTimeout, map[string]interface{}{
			"err": flushCtx.Err().Error(),
		})
	}
	ctx.PlainText(http.StatusOK, []byte("success"))
}

func doFlush(ctx context.Context, cancel context.CancelFunc, start, end time.Time, hasTimeout bool) error {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		mqs := queue.GetManager().ManagedQueues()
		wg := sync.WaitGroup{}
		wg.Add(len(mqs))
		allEmpty := true
		for _, mq := range mqs {
			if mq.IsEmpty() {
				wg.Done()
				continue
			}
			allEmpty = false
			if pool, ok := mq.Managed.(queue.WorkerPool); ok {
				go func() {
					localCtx, localCancel := context.WithCancel(ctx)
					pid := mq.RegisterWorkers(1, start, hasTimeout, end, localCancel, true)
					err := pool.FlushWithContext(localCtx)
					if err != nil && err != ctx.Err() {
						cancel()
					}
					mq.CancelWorkers(pid)
					localCancel()
					wg.Done()
				}()
			} else {
				wg.Done()
			}

		}
		if allEmpty {
			break
		}
		wg.Wait()
	}
	return nil
}
