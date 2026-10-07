// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePgSQLHostPort(t *testing.T) {
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
			host, port := parsePgSQLHostPort(test.HostPort)
			assert.Equal(t, test.Host, host)
			assert.Equal(t, test.Port, port)
		})
	}
}

func TestMakePgSQLConnStr(t *testing.T) {
	tests := []struct {
		Host    string
		User    string
		Passwd  string
		Name    string
		SSLMode string
		Output  string
	}{
		{
			Host:   "", // empty means default
			Output: "postgres://:@127.0.0.1:5432?sslmode=",
		},
		{
			Host:    "/tmp/pg.sock",
			User:    "testuser",
			Passwd:  "space space !#$%^^%^```-=?=",
			Name:    "gitea",
			SSLMode: "false",
			Output:  "postgres://testuser:space%20space%20%21%23$%25%5E%5E%25%5E%60%60%60-=%3F=@:5432/gitea?host=%2Ftmp%2Fpg.sock&sslmode=false",
		},
		{
			Host:    "/tmp/pg.sock:6432",
			User:    "testuser",
			Passwd:  "pass",
			Name:    "gitea",
			SSLMode: "false",
			Output:  "postgres://testuser:pass@:6432/gitea?host=%2Ftmp%2Fpg.sock&sslmode=false",
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
		connStr := makePgSQLConnStr(test.Host, test.User, test.Passwd, test.Name, test.SSLMode)
		assert.Equal(t, test.Output, connStr)
	}
}

func TestMakeFirebirdConnStr(t *testing.T) {
	tests := []struct {
		Host   string
		User   string
		Passwd string
		Name   string
		Output string
		HasErr bool
	}{
		{
			Host:   "", // empty means default
			Name:   "gitea",
			Output: "firebird://127.0.0.1:3050/gitea?default_query_exec_mode=exec&encoding=UTF8&default_transaction_iso_level=read_committed",
		},
		{
			Host:   "localhost",
			User:   "SYSDBA",
			Passwd: "masterkey",
			Name:   "gitea",
			Output: "firebird://SYSDBA:masterkey@localhost:3050/gitea?default_query_exec_mode=exec&encoding=UTF8&default_transaction_iso_level=read_committed",
		},
		{
			Host:   "fb.example.com:3051",
			User:   "gitea",
			Passwd: "space space !#$%^^%^```-=?=",
			Name:   "gitea",
			Output: "firebird://gitea:space%20space%20%21%23$%25%5E%5E%25%5E%60%60%60-=%3F=@fb.example.com:3051/gitea?default_query_exec_mode=exec&encoding=UTF8&default_transaction_iso_level=read_committed",
		},
		{
			// an absolute path must keep its leading slash, which fbx would otherwise strip
			Host:   "localhost:3050",
			Name:   "/var/lib/firebird/gitea.fdb",
			Output: "firebird://localhost:3050//var/lib/firebird/gitea.fdb?default_query_exec_mode=exec&encoding=UTF8&default_transaction_iso_level=read_committed",
		},
		{
			Name:   "",
			HasErr: true,
		},
	}

	for _, test := range tests {
		connStr, err := makeFirebirdConnStr(ConnOptions{
			Host: test.Host, User: test.User, Passwd: test.Passwd, Database: test.Name, Type: "firebird",
		})
		if test.HasErr {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.Equal(t, test.Output, connStr)
	}
}
