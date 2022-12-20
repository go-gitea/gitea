// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sync

import (
	"sync"

	"code.gitea.io/gitea/modules/nosql"
	"code.gitea.io/gitea/modules/setting"
	redsync "github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v8"
)

type Locker interface {
	Lock() error
	Unlock() (bool, error)
}

type LockService interface {
	NewLock(name string) Locker
}

type memoryLock struct {
	mutex *sync.Mutex
}

func (r *memoryLock) Lock() error {
	r.mutex.Lock()
	return nil
}

func (r *memoryLock) Unlock() (bool, error) {
	r.mutex.Lock()
	return true, nil
}

var _ Locker = &memoryLock{}

type memoryLockService struct {
	lockes sync.Map
}

var _ LockService = &memoryLockService{}

func NewMemoryLockService() *memoryLockService {
	return &memoryLockService{}
}

func (l *memoryLockService) NewLock(name string) Locker {
	lock, _ := l.lockes.LoadOrStore(name, &sync.Mutex{})
	return &memoryLock{mutex: lock.(*sync.Mutex)}
}

type redisLockService struct {
	rs *redsync.Redsync
}

var _ LockService = &redisLockService{}

func NewRedisLockService(connection string) *redisLockService {
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

func (r *redisLockService) NewLock(name string) Locker {
	return &redisLock{mutex: r.rs.NewMutex(name)}
}

func (r *redisLock) Lock() error {
	return r.mutex.Lock()
}

func (r *redisLock) Unlock() (bool, error) {
	return r.mutex.Unlock()
}

func GetLockService() LockService {
	if setting.Sync.LockServiceType == "redis" {
		return NewRedisLockService(setting.Sync.LockServiceConnStr)
	}
	return NewMemoryLockService()
}
