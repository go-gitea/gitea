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
	*baseQueueNotifiable
	length   int
	internal baseLevelQueuePushPoper
	muCommon *sync.Mutex
}

func (q *baseLevelQueueCommonImpl) PushItem(ctx context.Context, data []byte) error {
	_, err := backoffCall(ctx, backoffOptionsDefault(noNotifyChan, time.After(pushBlockTime)), func() (retry bool, ret any, err error) {
		if q.muCommon != nil {
			q.muCommon.Lock()
			defer q.muCommon.Unlock()
		}

		cnt := int(q.internal.Len())
		if cnt >= q.length {
			return true, nil, nil
		}
		retry, err = false, q.internal.RPush(data)
		if err == levelqueue.ErrAlreadyInQueue {
			err = ErrAlreadyInQueue
		}
		if err == nil {
			q.notifyPushItem()
		}
		return retry, nil, err
	})
	return err
}

func (q *baseLevelQueueCommonImpl) PopItem(ctx context.Context) ([]byte, error) {
	return backoffCall(ctx, backoffOptionsDefault(q.notifySignal, infiniteTimerC), func() (retry bool, data []byte, err error) {
		if q.muCommon != nil {
			q.muCommon.Lock()
			defer q.muCommon.Unlock()
		}

		data, err = q.internal.LPop()
		if err == levelqueue.ErrNotFound {
			return true, nil, nil
		}
		if err != nil {
			return false, nil, err
		}
		return false, data, nil
	})
}

func (q *baseLevelQueueCommonImpl) removeAllItems() error {
	if q.muCommon != nil {
		q.muCommon.Lock()
		defer q.muCommon.Unlock()
	}
	// In the past, levelqueue corrupted a lot, so we introduced a workaround
	// https://github.com/go-gitea/gitea/pull/24912 to reset the corrupted levelqueue.
	//
	// Now, the levelqueue package should have been fixed by https://gitea.com/lunny/levelqueue/pulls/8
	// So the workaround is no longer needed, and we can just pop all items from the queue like other queues.
	//
	// TODO: it's better to have a RemoveAll() or Reset() method in levelqueue
	for items := q.internal.Len(); items > 0; items-- {
		_, err := q.internal.LPop()
		if err == levelqueue.ErrNotFound {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func baseLevelQueueCommon(cfg *BaseConfig, mu *sync.Mutex, internal baseLevelQueuePushPoper) *baseLevelQueueCommonImpl {
	return &baseLevelQueueCommonImpl{length: cfg.Length, muCommon: mu, internal: internal, baseQueueNotifiable: newBaseQueueNotifiable()}
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
