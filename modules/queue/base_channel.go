// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"errors"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/container"
)

var errChannelClosed = errors.New("channel is closed")

type baseChannel struct {
	c   chan []byte
	set container.Set[string]
	mu  sync.Mutex

	isUnique bool
}

var _ baseQueue = (*baseChannel)(nil)

func newBaseChannelGeneric(cfg *BaseConfig, unique bool) (baseQueue, error) {
	return &baseChannel{
		c:   make(chan []byte, cfg.Length),
		set: container.Set[string]{},

		isUnique: unique,
	}, nil
}

func newBaseChannelSimple(cfg *BaseConfig) (baseQueue, error) {
	return newBaseChannelGeneric(cfg, false)
}

func newBaseChannelUnique(cfg *BaseConfig) (baseQueue, error) {
	return newBaseChannelGeneric(cfg, true)
}

func (q *baseChannel) PushItem(ctx context.Context, data []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.c == nil {
		return errChannelClosed
	}

	if q.isUnique {
		if q.set.Contains(string(data)) {
			return ErrAlreadyInQueue
		}
	}

	select {
	case q.c <- data:
		if q.isUnique {
			q.set.Add(string(data))
		}
		return nil
	case <-time.After(pushBlockTime):
		return context.DeadlineExceeded
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *baseChannel) PopItem(ctx context.Context) ([]byte, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	select {
	case data, ok := <-q.c:
		if !ok {
			return nil, errChannelClosed
		}
		q.set.Remove(string(data))
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (q *baseChannel) HasItem(ctx context.Context, data []byte) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.set.Contains(string(data)), nil
}

func (q *baseChannel) Len(ctx context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.c == nil {
		return 0, errChannelClosed
	}

	return len(q.c), nil
}

func (q *baseChannel) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	close(q.c)
	q.set = container.Set[string]{}

	return nil
}

func (q *baseChannel) RemoveAll(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.c != nil && len(q.c) > 0 {
		<-q.c
	}
	return nil
}
