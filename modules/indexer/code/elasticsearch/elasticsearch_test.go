// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexPos(t *testing.T) {
	startIdx, endIdx := indexPos("test index start and end", "start", "end")
	assert.EqualValues(t, 11, startIdx)
	assert.EqualValues(t, 24, endIdx)
}
