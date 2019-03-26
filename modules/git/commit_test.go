// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	// FIXME: since drone will only git clone -depth=50, this should be moved to recent commit id
	/*commitsCount, err := CommitsCount("", "22d3d029e6f7e6359f3a6fbe8b7827b579ac7445")
	assert.NoError(t, err)
	assert.Equal(t, int64(7287), commitsCount)*/
}

func TestGetFullCommitID(t *testing.T) {
	id, err := GetFullCommitID("", "22d3d029")
	assert.NoError(t, err)
	assert.Equal(t, "22d3d029e6f7e6359f3a6fbe8b7827b579ac7445", id)
}

func TestGetFullCommitIDError(t *testing.T) {
	id, err := GetFullCommitID("", "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}
