// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"sync"
	"sync/atomic"

	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/queue/lqinternal"

	"gitea.com/lunny/levelqueue"
	"github.com/syndtr/goleveldb/leveldb"
)

type baseLevelQueueUnique struct {
	internal atomic.Pointer[levelqueue.UniqueQueue]

	conn string
	cfg  *BaseConfig
	db   *leveldb.DB

	mu sync.Mutex // the levelqueue.UniqueQueue is not thread-safe, there is no mutex protecting the underlying queue&set together
}

var _ baseQueue = (*baseLevelQueueUnique)(nil)

func newBaseLevelQueueUnique(cfg *BaseConfig) (baseQueue, error) {
	conn, db, err := prepareLevelDB(cfg)
	if err != nil {
		return nil, err
	}
	q := &baseLevelQueueUnique{conn: conn, cfg: cfg, db: db}
	lq, err := levelqueue.NewUniqueQueue(db, []byte(cfg.QueueFullName), []byte(cfg.SetFullName), false)
	if err != nil {
		return nil, err
	}
	q.internal.Store(lq)
	return q, nil
}

func (q *baseLevelQueueUnique) PushItem(ctx context.Context, data []byte) error {
	c := baseLevelQueueCommon(q.cfg, &q.mu, func() baseLevelQueuePushPoper { return q.internal.Load() })
	return c.PushItem(ctx, data)
}

func (q *baseLevelQueueUnique) PopItem(ctx context.Context) ([]byte, error) {
	c := baseLevelQueueCommon(q.cfg, &q.mu, func() baseLevelQueuePushPoper { return q.internal.Load() })
	return c.PopItem(ctx)
}

func (q *baseLevelQueueUnique) HasItem(ctx context.Context, data []byte) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.internal.Load().Has(data)
}

func (q *baseLevelQueueUnique) Len(ctx context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int(q.internal.Load().Len()), nil
}

func (q *baseLevelQueueUnique) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	err := q.internal.Load().Close()
	q.db = nil // the db is not managed by us, it's managed by the nosql manager
	_ = nosql.GetManager().CloseLevelDB(q.conn)
	return err
}

func (q *baseLevelQueueUnique) RemoveAll(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	lqinternal.RemoveLevelQueueKeys(q.db, []byte(q.cfg.QueueFullName))
	lqinternal.RemoveLevelQueueKeys(q.db, []byte(q.cfg.SetFullName))
	lq, err := levelqueue.NewUniqueQueue(q.db, []byte(q.cfg.QueueFullName), []byte(q.cfg.SetFullName), false)
	if err != nil {
		return err
	}
	old := q.internal.Load()
	q.internal.Store(lq)
	_ = old.Close() // Not ideal for concurrency. Luckily, the levelqueue only sets its db=nil because it doesn't manage the db, so far so good
	return nil
}
