// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsIPPPrivate(t *testing.T) {
	cases := []struct {
		ip        string
		isPrivate bool
	}{
		// case 0
		{
			ip:        "127.0.0.1",
			isPrivate: false, // TODO: according to go, this isn't private?
		},
		// case 1
		{
			ip:        "127.1.2.3",
			isPrivate: false, // TODO: according to go, this isn't private?
		},
		// case 2
		{
			ip:        "10.255.255.0",
			isPrivate: true,
		},
		// case 3
		{
			ip:        "8.8.8.8",
			isPrivate: false,
		},
		// case 4
		{
			ip:        "::1",
			isPrivate: false, // TODO: according to go, this isn't private?
		},
		// case 4
		{
			ip:        "2a12:7c40::f00d",
			isPrivate: false,
		},
	}

	for n, c := range cases {
		i := net.ParseIP(c.ip)
		p := IsIPPrivate(i)
		assert.Equal(t, c.isPrivate, p, "case %d: should be equal", n)
	}
}
