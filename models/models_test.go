// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parsePostgreSQLHostPort(t *testing.T) {
	test := func(input, expectedHost, expectedPort string) {
		host, port := parsePostgreSQLHostPort(input)
		assert.Equal(t, expectedHost, host)
		assert.Equal(t, expectedPort, port)
	}
	test("127.0.0.1:1234", "127.0.0.1", "1234")
	test("127.0.0.1", "127.0.0.1", "5432")
	test("[::1]:1234", "[::1]", "1234")
	test("[::1]", "[::1]", "5432")
	test("/tmp/pg.sock:1234", "/tmp/pg.sock", "1234")
	test("/tmp/pg.sock", "/tmp/pg.sock", "5432")
}
