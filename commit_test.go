// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	commitsCount, _ := CommitsCount(".", "d86a90f801dbe279db095437a8c7ea42c60e8d98")
	assert.Equal(t, int64(3), commitsCount)
}
