// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"sync"

	"gitea.dev/modules/nosql"

	"gitea.com/lunny/levelqueue"
	"github.com/syndtr/goleveldb/leveldb"
)

type baseLevelQueueUnique struct {
	*baseLevelQueueCommonImpl
	internal *levelqueue.UniqueQueue

	conn string
	cfg  *BaseConfig
	db   *leveldb.DB

	muBase sync.Mutex // protects multi-step queue+set operations, e.g. the length check followed by a push
}

var (
	_ baseQueue                    = (*baseLevelQueueUnique)(nil)
	_ baseQueueNotifiableInterface = (*baseLevelQueueUnique)(nil)
)

func newBaseLevelQueueUnique(cfg *BaseConfig) (baseQueue, error) {
	conn, db, err := prepareLevelDB(cfg)
	if err != nil {
		return nil, err
	}
	q := &baseLevelQueueUnique{conn: conn, cfg: cfg, db: db}
	q.internal, err = levelqueue.NewUniqueQueue(db, []byte(cfg.QueueFullName), []byte(cfg.SetFullName), false)
	if err != nil {
		return nil, err
	}
	q.baseLevelQueueCommonImpl = baseLevelQueueCommon(q.cfg, &q.muBase, q.internal)
	return q, nil
}

func (q *baseLevelQueueUnique) HasItem(ctx context.Context, data []byte) (bool, error) {
	q.muBase.Lock()
	defer q.muBase.Unlock()
	return q.internal.Has(data)
}

func (q *baseLevelQueueUnique) Len(ctx context.Context) (int, error) {
	q.muBase.Lock()
	defer q.muBase.Unlock()
	return int(q.internal.Len()), nil
}

func (q *baseLevelQueueUnique) Close() error {
	q.muBase.Lock()
	defer q.muBase.Unlock()
	err := q.internal.Close()
	q.db = nil // the db is not managed by us, it's managed by the nosql manager
	_ = nosql.GetManager().CloseLevelDB(q.conn)
	return err
}

func (q *baseLevelQueueUnique) RemoveAll(ctx context.Context) error {
	q.muBase.Lock()
	defer q.muBase.Unlock()
	return q.internal.RemoveAll()
}
