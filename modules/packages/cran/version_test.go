// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"3.0.2", "2.5.2", 1},
		{"1.10", "1.9", 1},
		{"1.0-1", "1.0", 1},
		{"1.0-1", "1.0.1", 0},
		{"1.0", "1.0.0", 0},
		{"01.002", "1.2", 0},
		{"1.0.0.1", "1.0", 1},
		{"1.0.10", "1.0.2", 1},
		{"1.99999999999999999999", "1.9999999999999999999", 1},
	} {
		t.Run(tc.a+"/"+tc.b, func(t *testing.T) {
			assert.Equal(t, tc.want, CompareVersions(tc.a, tc.b))
			assert.Equal(t, -tc.want, CompareVersions(tc.b, tc.a))
		})
	}
}
