// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinFields(t *testing.T) {
	values := []string{"usr/bin/a", "usr/bin/b\n\n%FILES%\netc/cron.d/x", "usr/bin/c"}

	assert.Equal(t, "usr/bin/a\nusr/bin/c", joinFields(values))
	assert.Len(t, values, 3) // must not modify the caller's slice
}
