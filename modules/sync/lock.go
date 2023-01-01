// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sync

import (
	"sync"

	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/setting"

	redsync "github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/moby/locker"
)

type Locker interface {
	Lock() error
	Unlock() (bool, error)
}

type LockService interface {
	GetLock(name string) Locker
}

type memoryLock struct {
	locker *locker.Locker
	name   string
}

func (r *memoryLock) Lock() error {
	r.locker.Lock(r.name)
	return nil
}

func (r *memoryLock) Unlock() (bool, error) {
	return true, r.locker.Unlock(r.name)
}

var _ Locker = &memoryLock{}

type memoryLockService struct {
	locker *locker.Locker
}

var _ LockService = &memoryLockService{}

func newMemoryLockService() *memoryLockService {
	return &memoryLockService{
		locker: locker.New(),
	}
}

func (l *memoryLockService) GetLock(name string) Locker {
	return &memoryLock{
		locker: l.locker,
		name:   name,
	}
}

type redisLockService struct {
	rs *redsync.Redsync
}

var _ LockService = &redisLockService{}

func newRedisLockService(connection string) *redisLockService {
	client := nosql.GetManager().GetRedisClient(connection)

	pool := goredis.NewPool(client) // or, pool := redigo.NewPool(...)

	// Create an instance of redisync to be used to obtain a mutual exclusion
	// lock.
	rs := redsync.New(pool)

	return &redisLockService{
		rs: rs,
	}
}

type redisLock struct {
	mutex *redsync.Mutex
}

func (r *redisLockService) GetLock(name string) Locker {
	return &redisLock{mutex: r.rs.NewMutex(name)}
}

func (r *redisLock) Lock() error {
	return r.mutex.Lock()
}

func (r *redisLock) Unlock() (bool, error) {
	return r.mutex.Unlock()
}

var (
	syncOnce    sync.Once
	lockService LockService
)

func getLockService() LockService {
	syncOnce.Do(func() {
		if setting.Sync.LockServiceType == "redis" {
			lockService = newRedisLockService(setting.Sync.LockServiceConnStr)
		} else {
			lockService = newMemoryLockService()
		}
	})
	return lockService
}

func GetLock(name string) Locker {
	return getLockService().GetLock(name)
}
