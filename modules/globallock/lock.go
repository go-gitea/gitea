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
	Lock() error            // lock the resource and block until it is unlocked by the holder
	TryLock() (bool, error) // try to lock the resource and return immediately, first return value indicates if the lock was successful
	Unlock() (bool, error)  // only lock with no error and TryLock returned true with no error can be unlocked
}

type LockService interface {
	GetLocker(name string) Locker // create or get a locker by name, RemoveLocker should be called after the locker is no longer needed
	RemoveLocker(name string)     // remove a locker by name from the pool. This should be invoked affect locker is no longer needed, i.e. a pull request merged or closed
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

func (l *memoryLockService) GetLocker(name string) Locker {
	v, _ := l.syncMap.LoadOrStore(name, &memoryLock{})
	return v.(*memoryLock)
}

func (l *memoryLockService) RemoveLocker(name string) {
	l.syncMap.Delete(name)
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

func (r *redisLockService) GetLocker(name string) Locker {
	return &redisLock{mutex: r.rs.NewMutex(name)}
}

func (r *redisLockService) RemoveLocker(name string) {
	// Do nothing
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

func GetLocker(name string) Locker {
	return getLockService().GetLocker(name)
}
