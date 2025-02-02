// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nosql

import (
	"context"
	"strconv"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/process"

	"github.com/redis/go-redis/v9"
	"github.com/syndtr/goleveldb/leveldb"
)

var manager *Manager

// Manager is the nosql connection manager
type Manager struct {
	ctx      context.Context
	finished context.CancelFunc
	mutex    sync.Mutex

	RedisConnections   map[string]*redisClientHolder
	LevelDBConnections map[string]*levelDBHolder
}

type redisClientHolder struct {
	redis.UniversalClient
	name  []string
	count int64
}

func (r *redisClientHolder) Close() error {
	return manager.CloseRedisClient(r.name[0])
}

type levelDBHolder struct {
	name  []string
	count int64
	db    *leveldb.DB
}

func init() {
	_ = GetManager()
}

// GetManager returns a Manager and initializes one as singleton is there's none yet
func GetManager() *Manager {
	if manager == nil {
		ctx, _, finished := process.GetManager().AddTypedContext(context.Background(), "Service: NoSQL", process.SystemProcessType, false)
		manager = &Manager{
			ctx:                ctx,
			finished:           finished,
			RedisConnections:   make(map[string]*redisClientHolder),
			LevelDBConnections: make(map[string]*levelDBHolder),
		}
	}
	return manager
}

func valToTimeDuration(vs []string) (result time.Duration) {
	var err error
	for _, v := range vs {
		result, err = time.ParseDuration(v)
		if err != nil {
			var val int
			val, err = strconv.Atoi(v)
			result = time.Duration(val)
		}
		if err == nil {
			return result
		}
	}
	return result
}
