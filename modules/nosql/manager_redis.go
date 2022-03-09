// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nosql

import (
	"crypto/tls"
	"path"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
)

var replacer = strings.NewReplacer("_", "", "-", "")

// CloseRedisClient closes a redis client
func (m *Manager) CloseRedisClient(connection string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	client, ok := m.RedisConnections[connection]
	if !ok {
		connection = ToRedisURI(connection).String()
		client, ok = m.RedisConnections[connection]
	}
	if !ok {
		return nil
	}

	client.count--
	if client.count > 0 {
		return nil
	}

	for _, name := range client.name {
		delete(m.RedisConnections, name)
	}
	return client.UniversalClient.Close()
}

// GetRedisClient gets a redis client for a particular connection
func (m *Manager) GetRedisClient(connection string) redis.UniversalClient {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	client, ok := m.RedisConnections[connection]
	if ok {
		client.count++
		return client
	}

	uri := ToRedisURI(connection)
	client, ok = m.RedisConnections[uri.String()]
	if ok {
		client.count++
		return client
	}
	client = &redisClientHolder{
		name: []string{connection, uri.String()},
	}

	opts := &redis.UniversalOptions{}
	tlsConfig := &tls.Config{}

	// Handle username/password
	if password, ok := uri.User.Password(); ok {
		opts.Password = password
		// Username does not appear to be handled by redis.Options
		opts.Username = uri.User.Username()
	} else if uri.User.Username() != "" {
		// assume this is the password
		opts.Password = uri.User.Username()
	}

	// Now handle the uri query sets
	for k, v := range uri.Query() {
		switch replacer.Replace(strings.ToLower(k)) {
		case "addr":
			opts.Addrs = append(opts.Addrs, v...)
		case "addrs":
			opts.Addrs = append(opts.Addrs, strings.Split(v[0], ",")...)
		case "username":
			opts.Username = v[0]
		case "password":
			opts.Password = v[0]
		case "database":
			fallthrough
		case "db":
			opts.DB, _ = strconv.Atoi(v[0])
		case "maxretries":
			opts.MaxRetries, _ = strconv.Atoi(v[0])
		case "minretrybackoff":
			opts.MinRetryBackoff = valToTimeDuration(v)
		case "maxretrybackoff":
			opts.MaxRetryBackoff = valToTimeDuration(v)
		case "timeout":
			timeout := valToTimeDuration(v)
			if timeout != 0 {
				if opts.DialTimeout == 0 {
					opts.DialTimeout = timeout
				}
				if opts.ReadTimeout == 0 {
					opts.ReadTimeout = timeout
				}
			}
		case "dialtimeout":
			opts.DialTimeout = valToTimeDuration(v)
		case "readtimeout":
			opts.ReadTimeout = valToTimeDuration(v)
		case "writetimeout":
			opts.WriteTimeout = valToTimeDuration(v)
		case "poolsize":
			opts.PoolSize, _ = strconv.Atoi(v[0])
		case "minidleconns":
			opts.MinIdleConns, _ = strconv.Atoi(v[0])
		case "pooltimeout":
			opts.PoolTimeout = valToTimeDuration(v)
		case "idletimeout":
			opts.IdleTimeout = valToTimeDuration(v)
		case "idlecheckfrequency":
			opts.IdleCheckFrequency = valToTimeDuration(v)
		case "maxredirects":
			opts.MaxRedirects, _ = strconv.Atoi(v[0])
		case "readonly":
			opts.ReadOnly, _ = strconv.ParseBool(v[0])
		case "routebylatency":
			opts.RouteByLatency, _ = strconv.ParseBool(v[0])
		case "routerandomly":
			opts.RouteRandomly, _ = strconv.ParseBool(v[0])
		case "sentinelmasterid":
			fallthrough
		case "mastername":
			opts.MasterName = v[0]
		case "skipverify":
			fallthrough
		case "insecureskipverify":
			insecureSkipVerify, _ := strconv.ParseBool(v[0])
			tlsConfig.InsecureSkipVerify = insecureSkipVerify
		case "clientname":
			client.name = append(client.name, v[0])
		}
	}

	switch uri.Scheme {
	case "redis+sentinels":
		fallthrough
	case "rediss+sentinel":
		opts.TLSConfig = tlsConfig
		fallthrough
	case "redis+sentinel":
		if uri.Host != "" {
			opts.Addrs = append(opts.Addrs, strings.Split(uri.Host, ",")...)
		}
		if uri.Path != "" {
			if db, err := strconv.Atoi(uri.Path[1:]); err == nil {
				opts.DB = db
			}
		}

		client.UniversalClient = redis.NewFailoverClient(opts.Failover())
	case "redis+clusters":
		fallthrough
	case "rediss+cluster":
		opts.TLSConfig = tlsConfig
		fallthrough
	case "redis+cluster":
		if uri.Host != "" {
			opts.Addrs = append(opts.Addrs, strings.Split(uri.Host, ",")...)
		}
		if uri.Path != "" {
			if db, err := strconv.Atoi(uri.Path[1:]); err == nil {
				opts.DB = db
			}
		}
		client.UniversalClient = redis.NewClusterClient(opts.Cluster())
	case "redis+socket":
		simpleOpts := opts.Simple()
		simpleOpts.Network = "unix"
		simpleOpts.Addr = path.Join(uri.Host, uri.Path)
		client.UniversalClient = redis.NewClient(simpleOpts)
	case "rediss":
		opts.TLSConfig = tlsConfig
		fallthrough
	case "redis":
		if uri.Host != "" {
			opts.Addrs = append(opts.Addrs, strings.Split(uri.Host, ",")...)
		}
		if uri.Path != "" {
			if db, err := strconv.Atoi(uri.Path[1:]); err == nil {
				opts.DB = db
			}
		}
		client.UniversalClient = redis.NewClient(opts.Simple())
	default:
		return nil
	}

	for _, name := range client.name {
		m.RedisConnections[name] = client
	}

	client.count++

	return client
}
