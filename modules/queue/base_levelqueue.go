// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"

	"gitea.dev/modules/nosql"

	"gitea.com/lunny/levelqueue"
	"github.com/syndtr/goleveldb/leveldb"
)

type baseLevelQueue struct {
	*baseLevelQueueCommonImpl
	internal *levelqueue.Queue

	conn string
	cfg  *BaseConfig
	db   *leveldb.DB
}

var (
	_ baseQueue                    = (*baseLevelQueue)(nil)
	_ baseQueueNotifiableInterface = (*baseLevelQueue)(nil)
)

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
	q.internal, err = levelqueue.NewQueue(db, []byte(cfg.QueueFullName), false)
	if err != nil {
		return nil, err
	}
	q.baseLevelQueueCommonImpl = baseLevelQueueCommon(q.cfg, nil, q.internal)
	return q, nil
}

func (q *baseLevelQueue) HasItem(ctx context.Context, data []byte) (bool, error) {
	return false, nil
}

func (q *baseLevelQueue) Len(ctx context.Context) (int, error) {
	return int(q.internal.Len()), nil
}

func (q *baseLevelQueue) Close() error {
	err := q.internal.Close()
	_ = nosql.GetManager().CloseLevelDB(q.conn)
	q.db = nil // the db is not managed by us, it's managed by the nosql manager
	return err
}

func (q *baseLevelQueue) RemoveAll(ctx context.Context) error {
	return q.internal.RemoveAll()
}
