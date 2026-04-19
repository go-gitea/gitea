// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIframeSandboxSafeForSrc(t *testing.T) {
	cases := []struct {
		sandbox string
		safe    bool
	}{
		{"", false},
		{"allow-scripts", true},
		{"allow-scripts allow-forms allow-popups", true},
		{"allow-scripts allow-same-origin", false},
		{"allow-same-origin", false},
		{"  allow-scripts   allow-same-origin  ", false},
	}
	for _, tc := range cases {
		t.Run(tc.sandbox, func(t *testing.T) {
			assert.Equal(t, tc.safe, iframeSandboxSafeForSrc(tc.sandbox))
		})
	}
}
