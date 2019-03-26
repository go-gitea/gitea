// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	p, err := filepath.Abs("../../")
	assert.NoError(t, err)
	commitsCount, err := CommitsCount(p, "a30e5bcaf83a82f5f7d1c89a6f9f7e52036d74af")
	assert.NoError(t, err)
	assert.Equal(t, int64(6), commitsCount)
}

func TestGetFullCommitID(t *testing.T) {
	p, err := filepath.Abs("../../")
	assert.NoError(t, err)
	id, err := GetFullCommitID(p, "a30e5bca")
	assert.NoError(t, err)
	assert.Equal(t, "a30e5bcaf83a82f5f7d1c89a6f9f7e52036d74af", id)
}

func TestGetFullCommitIDError(t *testing.T) {
	p, err := filepath.Abs("../../")
	assert.NoError(t, err)
	id, err := GetFullCommitID(p, "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}
