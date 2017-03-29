// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parsePostgreSQLHostPort(t *testing.T) {
	tests := []struct {
		HostPort string
		Host     string
		Port     string
	}{
		{
			HostPort: "127.0.0.1:1234",
			Host:     "127.0.0.1",
			Port:     "1234",
		},
		{
			HostPort: "127.0.0.1",
			Host:     "127.0.0.1",
			Port:     "5432",
		},
		{
			HostPort: "[::1]:1234",
			Host:     "[::1]",
			Port:     "1234",
		},
		{
			HostPort: "[::1]",
			Host:     "[::1]",
			Port:     "5432",
		},
		{
			HostPort: "/tmp/pg.sock:1234",
			Host:     "/tmp/pg.sock",
			Port:     "1234",
		},
		{
			HostPort: "/tmp/pg.sock",
			Host:     "/tmp/pg.sock",
			Port:     "5432",
		},
	}
	for _, test := range tests {
		host, port := parsePostgreSQLHostPort(test.HostPort)
		assert.Equal(t, test.Host, host)
		assert.Equal(t, test.Port, port)
	}
}
