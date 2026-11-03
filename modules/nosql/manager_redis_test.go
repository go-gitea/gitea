// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nosql

import (
	"net/url"
	"testing"
)

func TestRedisOptions(t *testing.T) {
	for _, test := range []struct {
		name       string
		connection string
		valid      func(*url.URL) bool
	}{
		{"username", "redis://redis:password@myredis/0", func(uri *url.URL) bool { return getRedisOptions(uri).Username == "redis" }},
		{"password", "redis://redis:password@myredis/0", func(uri *url.URL) bool { return getRedisOptions(uri).Password == "password" }},
		{"skipverify", "rediss://myredis/0?skipverify=true", func(uri *url.URL) bool { return getRedisTLSOptions(uri).InsecureSkipVerify }},
		{"insecureskipverify", "rediss://myredis/0?insecureskipverify=true", func(uri *url.URL) bool { return getRedisTLSOptions(uri).InsecureSkipVerify }},
		{"sentinel username", "redis+sentinel://redis:password@myredis/0?sentinelusername=suser&sentinelpassword=spass", func(uri *url.URL) bool { return getRedisOptions(uri).Failover().SentinelUsername == "suser" }},
		{"sentinel password", "redis+sentinel://redis:password@myredis/0?sentinelusername=suser&sentinelpassword=spass", func(uri *url.URL) bool { return getRedisOptions(uri).Failover().SentinelPassword == "spass" }},
		{"tcp database", "redis://redis:password@myredis/12", func(uri *url.URL) bool { return getRedisOptions(uri).DB == 12 }},
		{"unix database", "redis+socket:///var/run/redis.sock?database=12", func(uri *url.URL) bool { return getRedisOptions(uri).DB == 12 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			uri, _ := url.Parse(test.connection)
			if !test.valid(uri) {
				t.Fail()
			}
		})
	}
}
