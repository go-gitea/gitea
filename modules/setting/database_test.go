// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parsePostgreSQLHostPort(t *testing.T) {
	tests := map[string]struct {
		HostPort string
		Host     string
		Port     string
	}{
		"host-port": {
			HostPort: "127.0.0.1:1234",
			Host:     "127.0.0.1",
			Port:     "1234",
		},
		"no-port": {
			HostPort: "127.0.0.1",
			Host:     "127.0.0.1",
			Port:     "5432",
		},
		"ipv6-port": {
			HostPort: "[::1]:1234",
			Host:     "::1",
			Port:     "1234",
		},
		"ipv6-no-port": {
			HostPort: "[::1]",
			Host:     "::1",
			Port:     "5432",
		},
		"unix-socket": {
			HostPort: "/tmp/pg.sock:1234",
			Host:     "/tmp/pg.sock",
			Port:     "1234",
		},
		"unix-socket-no-port": {
			HostPort: "/tmp/pg.sock",
			Host:     "/tmp/pg.sock",
			Port:     "5432",
		},
	}
	for k, test := range tests {
		t.Run(k, func(t *testing.T) {
			t.Log(test.HostPort)
			host, port := parsePostgreSQLHostPort(test.HostPort)
			assert.Equal(t, test.Host, host)
			assert.Equal(t, test.Port, port)
		})
	}
}

func Test_getPostgreSQLConnectionString(t *testing.T) {
	tests := []struct {
		Host    string
		User    string
		Passwd  string
		Name    string
		SSLMode string
		Output  string
	}{
		{
			Host:    "/tmp/pg.sock",
			User:    "testuser",
			Passwd:  "space space !#$%^^%^```-=?=",
			Name:    "gitea",
			SSLMode: "false",
			Output:  "postgres://testuser:space%20space%20%21%23$%25%5E%5E%25%5E%60%60%60-=%3F=@:5432/gitea?host=%2Ftmp%2Fpg.sock&sslmode=false",
		},
		{
			Host:    "localhost",
			User:    "pgsqlusername",
			Passwd:  "I love Gitea!",
			Name:    "gitea",
			SSLMode: "true",
			Output:  "postgres://pgsqlusername:I%20love%20Gitea%21@localhost:5432/gitea?sslmode=true",
		},
		{
			Host:   "localhost:1234",
			User:   "user",
			Passwd: "pass",
			Name:   "gitea?param=1",
			Output: "postgres://user:pass@localhost:1234/gitea?param=1&sslmode=",
		},
	}

	for _, test := range tests {
		connStr := getPostgreSQLConnectionString(test.Host, test.User, test.Passwd, test.Name, test.SSLMode)
		assert.Equal(t, test.Output, connStr)
	}
}
