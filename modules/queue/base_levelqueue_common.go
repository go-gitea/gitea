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

	"code.gitea.io/gitea/modules/nosql"

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
}

func (q *baseLevelQueueCommonImpl) PushItem(ctx context.Context, data []byte) error {
	return backoffErr(ctx, backoffBegin, backoffUpper, time.After(pushBlockTime), func() (retry bool, err error) {
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
}

func (q *baseLevelQueueCommonImpl) PopItem(ctx context.Context) ([]byte, error) {
	return backoffRetErr(ctx, backoffBegin, backoffUpper, infiniteTimerC, func() (retry bool, data []byte, err error) {
		if q.mu != nil {
			q.mu.Lock()
			defer q.mu.Unlock()
		}

		data, err = q.internalFunc().LPop()
		if err == levelqueue.ErrNotFound {
			return true, nil, nil
		}
		if err != nil {
			return false, nil, err
		}
		return false, data, nil
	})
}

func baseLevelQueueCommon(cfg *BaseConfig, mu *sync.Mutex, internalFunc func() baseLevelQueuePushPoper) *baseLevelQueueCommonImpl {
	return &baseLevelQueueCommonImpl{length: cfg.Length, mu: mu, internalFunc: internalFunc}
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
	for i := 0; i < 10; i++ {
		if db, err = nosql.GetManager().GetLevelDB(conn); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	return conn, db, err
}
