// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package globallock

import (
	"context"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/setting"

	redsync "github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
)

type Locker interface {
	Lock() error
	TryLock() (bool, error)
	Unlock() (bool, error)
}

type LockService interface {
	GetLock(name string) Locker
}

type memoryLock struct {
	mutex sync.Mutex
}

func (r *memoryLock) Lock() error {
	r.mutex.Lock()
	return nil
}

func (r *memoryLock) TryLock() (bool, error) {
	return r.mutex.TryLock(), nil
}

func (r *memoryLock) Unlock() (bool, error) {
	r.mutex.Unlock()
	return true, nil
}

var _ Locker = &memoryLock{}

type memoryLockService struct {
	syncMap sync.Map
}

var _ LockService = &memoryLockService{}

func newMemoryLockService() *memoryLockService {
	return &memoryLockService{
		syncMap: sync.Map{},
	}
}

func (l *memoryLockService) GetLock(name string) Locker {
	v, _ := l.syncMap.LoadOrStore(name, &memoryLock{})
	return v.(*memoryLock)
}

type redisLockService struct {
	rs *redsync.Redsync
}

var _ LockService = &redisLockService{}

func newRedisLockService(connection string) *redisLockService {
	client := nosql.GetManager().GetRedisClient(connection)

	pool := goredis.NewPool(client)

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

func (r *redisLock) TryLock() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := r.mutex.LockContext(ctx); err != nil {
		return false, err
	}
	return true, nil
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
		if setting.GlobalLock.ServiceType == "redis" {
			lockService = newRedisLockService(setting.GlobalLock.ServiceConnStr)
		} else {
			lockService = newMemoryLockService()
		}
	})
	return lockService
}

func GetLock(name string) Locker {
	return getLockService().GetLock(name)
}
