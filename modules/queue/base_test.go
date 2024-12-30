// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testQueueBasic(t *testing.T, newFn func(cfg *BaseConfig) (baseQueue, error), cfg *BaseConfig, isUnique bool) {
	t.Run(fmt.Sprintf("testQueueBasic-%s-unique:%v", cfg.ManagedName, isUnique), func(t *testing.T) {
		q, err := newFn(cfg)
		assert.NoError(t, err)

		ctx := context.Background()
		_ = q.RemoveAll(ctx)
		cnt, err := q.Len(ctx)
		assert.NoError(t, err)
		assert.EqualValues(t, 0, cnt)

		// push the first item
		err = q.PushItem(ctx, []byte("foo"))
		assert.NoError(t, err)

		cnt, err = q.Len(ctx)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, cnt)

		// push a duplicate item
		err = q.PushItem(ctx, []byte("foo"))
		if !isUnique {
			assert.NoError(t, err)
		} else {
			assert.ErrorIs(t, err, ErrAlreadyInQueue)
		}

		// check the duplicate item
		cnt, err = q.Len(ctx)
		assert.NoError(t, err)
		has, err := q.HasItem(ctx, []byte("foo"))
		assert.NoError(t, err)
		if !isUnique {
			assert.EqualValues(t, 2, cnt)
			assert.False(t, has) // non-unique queues don't check for duplicates
		} else {
			assert.EqualValues(t, 1, cnt)
			assert.True(t, has)
		}

		// push another item
		err = q.PushItem(ctx, []byte("bar"))
		assert.NoError(t, err)

		// pop the first item (and the duplicate if non-unique)
		it, err := q.PopItem(ctx)
		assert.NoError(t, err)
		assert.EqualValues(t, "foo", string(it))

		if !isUnique {
			it, err = q.PopItem(ctx)
			assert.NoError(t, err)
			assert.EqualValues(t, "foo", string(it))
		}

		// pop another item
		it, err = q.PopItem(ctx)
		assert.NoError(t, err)
		assert.EqualValues(t, "bar", string(it))

		// pop an empty queue (timeout, cancel)
		ctxTimed, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		it, err = q.PopItem(ctxTimed)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Nil(t, it)
		cancel()

		ctxTimed, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
		cancel()
		it, err = q.PopItem(ctxTimed)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, it)

		// test blocking push if queue is full
		for i := 0; i < cfg.Length; i++ {
			err = q.PushItem(ctx, []byte(fmt.Sprintf("item-%d", i)))
			assert.NoError(t, err)
		}
		ctxTimed, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
		err = q.PushItem(ctxTimed, []byte("item-full"))
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		cancel()

		// test blocking push if queue is full (with custom pushBlockTime)
		oldPushBlockTime := pushBlockTime
		timeStart := time.Now()
		pushBlockTime = 30 * time.Millisecond
		err = q.PushItem(ctx, []byte("item-full"))
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.GreaterOrEqual(t, time.Since(timeStart), pushBlockTime*2/3)
		pushBlockTime = oldPushBlockTime

		// remove all
		cnt, err = q.Len(ctx)
		assert.NoError(t, err)
		assert.EqualValues(t, cfg.Length, cnt)

		_ = q.RemoveAll(ctx)

		cnt, err = q.Len(ctx)
		assert.NoError(t, err)
		assert.EqualValues(t, 0, cnt)
	})
}

func TestBaseDummy(t *testing.T) {
	q, err := newBaseDummy(&BaseConfig{}, true)
	assert.NoError(t, err)

	ctx := context.Background()
	assert.NoError(t, q.PushItem(ctx, []byte("foo")))

	cnt, err := q.Len(ctx)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, cnt)

	has, err := q.HasItem(ctx, []byte("foo"))
	assert.NoError(t, err)
	assert.False(t, has)

	it, err := q.PopItem(ctx)
	assert.NoError(t, err)
	assert.Nil(t, it)

	assert.NoError(t, q.RemoveAll(ctx))
}
