// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexPos(t *testing.T) {
	startIdx, endIdx := contentMatchIndexPos("test index start and end", "start", "end")
	assert.Equal(t, 11, startIdx)
	assert.Equal(t, 15, endIdx)
}
