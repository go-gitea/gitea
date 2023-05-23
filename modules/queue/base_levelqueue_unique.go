// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"sync"
	"unsafe"

	"code.gitea.io/gitea/modules/nosql"

	"gitea.com/lunny/levelqueue"
	"github.com/syndtr/goleveldb/leveldb"
)

type baseLevelQueueUnique struct {
	internal *levelqueue.UniqueQueue
	conn     string
	cfg      *BaseConfig

	mu sync.Mutex // the levelqueue.UniqueQueue is not thread-safe, there is no mutex protecting the underlying queue&set together
}

var _ baseQueue = (*baseLevelQueueUnique)(nil)

func newBaseLevelQueueUnique(cfg *BaseConfig) (baseQueue, error) {
	conn, db, err := prepareLevelDB(cfg)
	if err != nil {
		return nil, err
	}
	q := &baseLevelQueueUnique{conn: conn, cfg: cfg}
	q.internal, err = levelqueue.NewUniqueQueue(db, []byte(cfg.QueueFullName), []byte(cfg.SetFullName), false)
	if err != nil {
		return nil, err
	}

	return q, nil
}

func (q *baseLevelQueueUnique) PushItem(ctx context.Context, data []byte) error {
	return baseLevelQueueCommon(q.cfg, q.internal, &q.mu).PushItem(ctx, data)
}

func (q *baseLevelQueueUnique) PopItem(ctx context.Context) ([]byte, error) {
	return baseLevelQueueCommon(q.cfg, q.internal, &q.mu).PopItem(ctx)
}

func (q *baseLevelQueueUnique) HasItem(ctx context.Context, data []byte) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.internal.Has(data)
}

func (q *baseLevelQueueUnique) Len(ctx context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int(q.internal.Len()), nil
}

func (q *baseLevelQueueUnique) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	err := q.internal.Close()
	_ = nosql.GetManager().CloseLevelDB(q.conn)
	return err
}

func (q *baseLevelQueueUnique) RemoveAll(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	type levelUniqueQueue struct {
		q   *levelqueue.Queue
		set *levelqueue.Set
		db  *leveldb.DB
	}
	lq := (*levelUniqueQueue)(unsafe.Pointer(q.internal))

	for lq.q.Len() > 0 {
		if _, err := lq.q.LPop(); err != nil {
			return err
		}
	}

	// the "set" must be cleared after the "list" because there is no transaction.
	// it's better to have duplicate items than losing items.
	members, err := lq.set.Members()
	if err != nil {
		return err // seriously corrupted
	}
	for _, v := range members {
		_, _ = lq.set.Remove(v)
	}
	return nil
}
