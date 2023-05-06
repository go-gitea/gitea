// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsUseLightColor(t *testing.T) {
	cases := []struct {
		r        float64
		g        float64
		b        float64
		expected bool
	}{
		{215, 58, 74, true},
		{0, 117, 202, true},
		{207, 211, 215, false},
		{162, 238, 239, false},
		{112, 87, 255, true},
		{0, 134, 114, true},
		{228, 230, 105, false},
		{216, 118, 227, false},
		{255, 255, 255, false},
		{43, 134, 133, true},
		{43, 135, 134, true},
		{44, 135, 134, false},
		{59, 182, 179, false},
		{124, 114, 104, true},
		{126, 113, 108, true},
		{129, 112, 109, true},
		{128, 112, 112, true},
	}
	for n, c := range cases {
		result := IsUseLightColor(c.r, c.g, c.b)
		assert.Equal(t, c.expected, result, "case %d: error should match", n)
	}
}
