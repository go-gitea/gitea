// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"time"

	"code.gitea.io/gitea/core"
)

type scheduler struct {
	*queue
}

// New creates a new scheduler.
func New() core.Scheduler {
	return scheduler{
		queue: newQueue(),
	}
}

// newQueue returns a new Queue backed by the build datastore.
func newQueue() *queue {
	q := &queue{
		ready:    make(chan struct{}, 1),
		workers:  map[*worker]struct{}{},
		interval: time.Minute,
		ctx:      context.Background(),
	}
	go func() {
		_ = q.start()
	}()
	return q
}
