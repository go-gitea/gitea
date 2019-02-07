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

func TestGetFullCommitID(t *testing.T) {
	id, err := GetFullCommitID(".", "d86a90f8")
	assert.NoError(t, err)
	assert.Equal(t, "d86a90f801dbe279db095437a8c7ea42c60e8d98", id)
}

func TestGetFullCommitIDError(t *testing.T) {
	id, err := GetFullCommitID(".", "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}
