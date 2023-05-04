// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"sync"
	"time"

	"gitea.com/lunny/levelqueue"
)

type baseLevelQueuePushPoper interface {
	RPush(data []byte) error
	LPop() ([]byte, error)
	Len() int64
}

type baseLevelQueueCommonImpl struct {
	length   int
	internal baseLevelQueuePushPoper
	mu       *sync.Mutex
}

func (q *baseLevelQueueCommonImpl) PushItem(ctx context.Context, data []byte) error {
	return backoffErr(ctx, backoffBegin, backoffUpper, time.After(pushBlockTime), func() (retry bool, err error) {
		if q.mu != nil {
			q.mu.Lock()
			defer q.mu.Unlock()
		}

		cnt := int(q.internal.Len())
		if cnt >= q.length {
			return true, nil
		}
		retry, err = false, q.internal.RPush(data)
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

func baseLevelQueueCommon(cfg *BaseConfig, internal baseLevelQueuePushPoper, mu *sync.Mutex) *baseLevelQueueCommonImpl {
	return &baseLevelQueueCommonImpl{length: cfg.Length, internal: internal}
}
