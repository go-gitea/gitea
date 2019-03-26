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
	p, err := filepath.Abs(".")
	assert.NoError(t, err)
	commitsCount, err := CommitsCount(p, "4836fea8767c38f175f59f8f66579e76fe6354f5")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), commitsCount)
}

func TestGetFullCommitID(t *testing.T) {
	p, err := filepath.Abs(".")
	assert.NoError(t, err)
	id, err := GetFullCommitID(p, "4836fea8")
	assert.NoError(t, err)
	assert.Equal(t, "4836fea8767c38f175f59f8f66579e76fe6354f5", id)
}

func TestGetFullCommitIDError(t *testing.T) {
	p, err := filepath.Abs(".")
	assert.NoError(t, err)
	id, err := GetFullCommitID(p, "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}
