// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nosql

import (
	"net/url"
	"testing"
)

func TestRedisUsernameOpt(t *testing.T) {
	uri, _ := url.Parse("redis://redis:password@myredis/0")
	opts := getRedisOptions(uri)

	if opts.Username != "redis" {
		t.Fail()
	}
}

func TestRedisPasswordOpt(t *testing.T) {
	uri, _ := url.Parse("redis://redis:password@myredis/0")
	opts := getRedisOptions(uri)

	if opts.Password != "password" {
		t.Fail()
	}
}

func TestSkipVerifyOpt(t *testing.T) {
	uri, _ := url.Parse("rediss://myredis/0?skipverify=true")
	tlsConfig := getRedisTLSOptions(uri)

	if !tlsConfig.InsecureSkipVerify {
		t.Fail()
	}
}

func TestInsecureSkipVerifyOpt(t *testing.T) {
	uri, _ := url.Parse("rediss://myredis/0?insecureskipverify=true")
	tlsConfig := getRedisTLSOptions(uri)

	if !tlsConfig.InsecureSkipVerify {
		t.Fail()
	}
}

func TestRedisSentinelUsernameOpt(t *testing.T) {
	uri, _ := url.Parse("redis+sentinel://redis:password@myredis/0?sentinelusername=suser&sentinelpassword=spass")
	opts := getRedisOptions(uri).Failover()

	if opts.SentinelUsername != "suser" {
		t.Fail()
	}
}

func TestRedisSentinelPasswordOpt(t *testing.T) {
	uri, _ := url.Parse("redis+sentinel://redis:password@myredis/0?sentinelusername=suser&sentinelpassword=spass")
	opts := getRedisOptions(uri).Failover()

	if opts.SentinelPassword != "spass" {
		t.Fail()
	}
}

func TestRedisDatabaseIndexTcp(t *testing.T) {
	uri, _ := url.Parse("redis://redis:password@myredis/12")
	opts := getRedisOptions(uri)

	if opts.DB != 12 {
		t.Fail()
	}
}

func TestRedisDatabaseIndexUnix(t *testing.T) {
	uri, _ := url.Parse("redis+socket:///var/run/redis.sock?database=12")
	opts := getRedisOptions(uri)

	if opts.DB != 12 {
		t.Fail()
	}
}
