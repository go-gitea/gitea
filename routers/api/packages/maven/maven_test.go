// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package maven

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChecksumSizeAllowed(t *testing.T) {
	assert.True(t, isChecksumSizeAllowed(maxChecksumSize))
	assert.False(t, isChecksumSizeAllowed(maxChecksumSize+1))
}
