// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/core"
	"code.gitea.io/gitea/models/bots"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

type worker struct {
	kind    string
	typ     string
	os      string
	arch    string
	channel chan *runnerv1.Stage
}

type queue struct {
	sync.Mutex

	ready    chan struct{}
	paused   bool
	interval time.Duration
	workers  map[*worker]struct{}
	ctx      context.Context
}

func (q *queue) Schedule(ctx context.Context, stage *runnerv1.Stage) error {
	select {
	case q.ready <- struct{}{}:
	default:
	}
	return nil
}

func (q *queue) Request(ctx context.Context, params core.Filter) (*runnerv1.Stage, error) {
	w := &worker{
		kind:    params.Kind,
		typ:     params.Type,
		os:      params.OS,
		arch:    params.Arch,
		channel: make(chan *runnerv1.Stage),
	}
	q.Lock()
	q.workers[w] = struct{}{}
	q.Unlock()

	select {
	case q.ready <- struct{}{}:
	default:
	}

	select {
	case <-ctx.Done():
		q.Lock()
		delete(q.workers, w)
		q.Unlock()
		return nil, ctx.Err()
	case b := <-w.channel:
		return b, nil
	}
}

func (q *queue) start() error {
	for {
		select {
		case <-q.ctx.Done():
			return q.ctx.Err()
		case <-q.ready:
			_ = q.signal(q.ctx)
		case <-time.After(q.interval):
			_ = q.signal(q.ctx)
		}
	}
}

func (q *queue) signal(ctx context.Context) error {
	q.Lock()
	count := len(q.workers)
	pause := q.paused
	q.Unlock()
	if pause {
		return nil
	}
	if count == 0 {
		return nil
	}
	items, err := bots.FindStages(ctx, bots.FindStageOptions{})
	if err != nil {
		return err
	}

	q.Lock()
	defer q.Unlock()
	for _, item := range items {
		if item.Status == core.StatusRunning {
			continue
		}
		if item.Machine != "" {
			continue
		}

	loop:
		for w := range q.workers {
			// the worker must match the resource kind and type
			if !matchResource(w.kind, w.typ, item.Kind, item.Type) {
				continue
			}

			if w.os != "" || w.arch != "" {
				if w.os != item.OS {
					continue
				}
				if w.arch != item.Arch {
					continue
				}
			}

			stage := &runnerv1.Stage{
				Id:      item.ID,
				BuildId: item.BuildID,
				Name:    item.Name,
				Kind:    item.Name,
				Type:    item.Type,
				Status:  string(item.Status),
				Started: int64(item.Started),
				Stopped: int64(item.Stopped),
			}

			w.channel <- stage
			delete(q.workers, w)
			break loop
		}
	}
	return nil
}

// matchResource is a helper function that returns
func matchResource(kinda, typea, kindb, typeb string) bool {
	if kinda == "" {
		kinda = "pipeline"
	}
	if kindb == "" {
		kindb = "pipeline"
	}
	if typea == "" {
		typea = "docker"
	}
	if typeb == "" {
		typeb = "docker"
	}
	return kinda == kindb && typea == typeb
}
