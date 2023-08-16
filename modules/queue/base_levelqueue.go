// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"sync/atomic"

	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/queue/lqinternal"

	"gitea.com/lunny/levelqueue"
	"github.com/syndtr/goleveldb/leveldb"
)

type baseLevelQueue struct {
	internal atomic.Pointer[levelqueue.Queue]

	conn string
	cfg  *BaseConfig
	db   *leveldb.DB
}

var _ baseQueue = (*baseLevelQueue)(nil)

func newBaseLevelQueueGeneric(cfg *BaseConfig, unique bool) (baseQueue, error) {
	if unique {
		return newBaseLevelQueueUnique(cfg)
	}
	return newBaseLevelQueueSimple(cfg)
}

func newBaseLevelQueueSimple(cfg *BaseConfig) (baseQueue, error) {
	conn, db, err := prepareLevelDB(cfg)
	if err != nil {
		return nil, err
	}
	q := &baseLevelQueue{conn: conn, cfg: cfg, db: db}
	lq, err := levelqueue.NewQueue(db, []byte(cfg.QueueFullName), false)
	if err != nil {
		return nil, err
	}
	q.internal.Store(lq)
	return q, nil
}

func (q *baseLevelQueue) PushItem(ctx context.Context, data []byte) error {
	c := baseLevelQueueCommon(q.cfg, nil, func() baseLevelQueuePushPoper { return q.internal.Load() })
	return c.PushItem(ctx, data)
}

func (q *baseLevelQueue) PopItem(ctx context.Context) ([]byte, error) {
	c := baseLevelQueueCommon(q.cfg, nil, func() baseLevelQueuePushPoper { return q.internal.Load() })
	return c.PopItem(ctx)
}

func (q *baseLevelQueue) HasItem(ctx context.Context, data []byte) (bool, error) {
	return false, nil
}

func (q *baseLevelQueue) Len(ctx context.Context) (int, error) {
	return int(q.internal.Load().Len()), nil
}

func (q *baseLevelQueue) Close() error {
	err := q.internal.Load().Close()
	_ = nosql.GetManager().CloseLevelDB(q.conn)
	q.db = nil // the db is not managed by us, it's managed by the nosql manager
	return err
}

func (q *baseLevelQueue) RemoveAll(ctx context.Context) error {
	lqinternal.RemoveLevelQueueKeys(q.db, []byte(q.cfg.QueueFullName))
	lq, err := levelqueue.NewQueue(q.db, []byte(q.cfg.QueueFullName), false)
	if err != nil {
		return err
	}
	old := q.internal.Load()
	q.internal.Store(lq)
	_ = old.Close() // Not ideal for concurrency. Luckily, the levelqueue only sets its db=nil because it doesn't manage the db, so far so good
	return nil
}
