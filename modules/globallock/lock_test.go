// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Lock(t *testing.T) {
	locker1 := GetLocker("test2")
	assert.NoError(t, locker1.Lock())
	unlocked, err := locker1.Unlock()
	assert.NoError(t, err)
	assert.True(t, unlocked)

	locker2 := GetLocker("test2")
	assert.NoError(t, locker2.Lock())

	locked1, err1 := locker2.TryLock()
	assert.NoError(t, err1)
	assert.False(t, locked1)

	locker2.Unlock()

	locked2, err2 := locker2.TryLock()
	assert.NoError(t, err2)
	assert.True(t, locked2)

	locker2.Unlock()
}

func Test_Lock_Redis(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("Skip test for local development")
	}

	lockService = newRedisLockService("redis://redis")

	redisPool :=
	locker1 := GetLocker("test1")
	assert.NoError(t, locker1.Lock())
	unlocked, err := locker1.Unlock()
	assert.NoError(t, err)
	assert.True(t, unlocked)

	locker2 := GetLocker("test1")
	assert.NoError(t, locker2.Lock())

	locked1, err1 := locker2.TryLock()
	assert.NoError(t, err1)
	assert.False(t, locked1)

	locker2.Unlock()

	locked2, err2 := locker2.TryLock()
	assert.NoError(t, err2)
	assert.True(t, locked2)

	locker2.Unlock()

	redisPool.Close()
}
