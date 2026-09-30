// Copyright 2013 Beego Authors
// Copyright 2014 The Macaron Authors
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"errors"
	"time"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/nosql"

	"github.com/redis/go-redis/v9"
)

type redisBackend struct {
	client      redis.UniversalClient
	prefix      string
	maxLifetime time.Duration
}

// newRedisBackend accepts a connection string like "redis://127.0.0.1:6379/0?prefix=session"
func newRedisBackend(config string, maxLifetime int64) (*redisBackend, error) {
	uri := nosql.ToRedisURI(config)
	b := &redisBackend{
		client:      nosql.GetManager().GetRedisClient(uri.String()),
		prefix:      uri.Query().Get("prefix"),
		maxLifetime: time.Duration(maxLifetime) * time.Second,
	}
	return b, b.client.Ping(graceful.GetManager().ShutdownContext()).Err()
}

func (b *redisBackend) load(sid string) ([]byte, error) {
	ctx := graceful.GetManager().HammerContext()
	var get *redis.StringCmd
	_, err := b.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		get = pipe.Get(ctx, b.prefix+sid)
		pipe.Expire(ctx, b.prefix+sid, b.maxLifetime)
		return nil
	})
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return get.Bytes()
}

func (b *redisBackend) save(sid string, data []byte, create bool) error {
	ctx := graceful.GetManager().HammerContext()
	if create {
		return b.client.Set(ctx, b.prefix+sid, data, b.maxLifetime).Err()
	}
	return b.client.SetXX(ctx, b.prefix+sid, data, b.maxLifetime).Err()
}

func (*redisBackend) touch(string) error {
	return nil
}

func (b *redisBackend) destroy(sid string) error {
	return b.client.Del(graceful.GetManager().HammerContext(), b.prefix+sid).Err()
}

func (*redisBackend) gc() {}
