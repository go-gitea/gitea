// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"net/http"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/queue"

	"gitea.com/macaron/macaron"
)

// FlushQueues flushes all the Queues
func FlushQueues(ctx *macaron.Context, opts private.FlushOptions) {
	if opts.NonBlocking {
		// Save the hammer ctx here - as a new one is created each time you call this.
		baseCtx := graceful.GetManager().HammerContext()
		go func() {
			err := queue.GetManager().FlushAll(baseCtx, opts.Timeout)
			if err != nil {
				log.Error("Flushing request timed-out with error: %v", err)
			}
		}()
		ctx.JSON(http.StatusAccepted, map[string]interface{}{
			"err": "Flushing",
		})
		return
	}
	err := queue.GetManager().FlushAll(ctx.Req.Request.Context(), opts.Timeout)
	if err != nil {
		ctx.JSON(http.StatusRequestTimeout, map[string]interface{}{
			"err": err,
		})
	}
	ctx.PlainText(http.StatusOK, []byte("success"))
}
