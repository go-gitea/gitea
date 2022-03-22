// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalPathIsValid(t *testing.T) {
	kases := []struct {
		path  string
		valid bool
	}{
		{
			"a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			true,
		},
		{
			"../a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			false,
		},
		{
			"a\\0\\a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			true,
		},
		{
			"b/../a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			false,
		},
		{
			"..\\a/0/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a14",
			false,
		},
	}

	for _, k := range kases {
		t.Run(k.path, func(t *testing.T) {
			assert.EqualValues(t, k.valid, isLocalPathValid(k.path))
		})
	}
}
