// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gitea.dev/modules/nosql"

	"gitea.com/lunny/levelqueue"
	"github.com/syndtr/goleveldb/leveldb"
)

// baseLevelQueuePushPoper is the common interface for levelqueue.Queue and levelqueue.UniqueQueue
type baseLevelQueuePushPoper interface {
	RPush(data []byte) error
	LPop() ([]byte, error)
	Len() int64
}

type baseLevelQueueCommonImpl struct {
	length       int
	internalFunc func() baseLevelQueuePushPoper
	mu           *sync.Mutex
	pushed       chan struct{}
}

func (q *baseLevelQueueCommonImpl) PushItem(ctx context.Context, data []byte) error {
	err := backoffErr(ctx, func() (retry bool, err error) {
		if q.mu != nil {
			q.mu.Lock()
			defer q.mu.Unlock()
		}

		cnt := int(q.internalFunc().Len())
		if cnt >= q.length {
			return true, nil
		}
		retry, err = false, q.internalFunc().RPush(data)
		if err == levelqueue.ErrAlreadyInQueue {
			err = ErrAlreadyInQueue
		}
		return retry, err
	})
	if err == nil {
		signalPush(q.pushed)
	}
	return err
}

func (q *baseLevelQueueCommonImpl) PopItem(ctx context.Context) ([]byte, error) {
	for {
		data, err := q.tryPopItem()
		if err != levelqueue.ErrNotFound {
			return data, err
		}
		if err := waitForPush(ctx, q.pushed); err != nil {
			return nil, err
		}
	}
}

func (q *baseLevelQueueCommonImpl) tryPopItem() ([]byte, error) {
	if q.mu != nil {
		q.mu.Lock()
		defer q.mu.Unlock()
	}
	return q.internalFunc().LPop()
}

func baseLevelQueueCommon(cfg *BaseConfig, mu *sync.Mutex, internalFunc func() baseLevelQueuePushPoper) *baseLevelQueueCommonImpl {
	return &baseLevelQueueCommonImpl{length: cfg.Length, mu: mu, internalFunc: internalFunc, pushed: make(chan struct{}, 1)}
}

func prepareLevelDB(cfg *BaseConfig) (conn string, db *leveldb.DB, err error) {
	if cfg.ConnStr == "" { // use data dir as conn str
		if !filepath.IsAbs(cfg.DataFullDir) {
			return "", nil, fmt.Errorf("invalid leveldb data dir (not absolute): %q", cfg.DataFullDir)
		}
		conn = cfg.DataFullDir
	} else {
		if !strings.HasPrefix(cfg.ConnStr, "leveldb://") {
			return "", nil, fmt.Errorf("invalid leveldb connection string: %q", cfg.ConnStr)
		}
		conn = cfg.ConnStr
	}
	for range 10 {
		if db, err = nosql.GetManager().GetLevelDB(conn); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return conn, db, err
}
