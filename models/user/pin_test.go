// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestUserPinUnpinRepos(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	// User:2 pins repositories 1 and 2
	{
		assert.NoError(t, PinRepos(2, 1, 2))
		pinned, err := GetPinnedRepositoryIDs(2)

		if assert.NoError(t, err) {
			expected := []int64{1, 2}
			assert.Equal(t, pinned, expected)
		}
	}
	// User:2 unpins repository 2, leaving just 1
	{
		assert.NoError(t, UnpinRepos(2, 1))

		pinned, err := GetPinnedRepositoryIDs(2)

		if assert.NoError(t, err) {
			expected := []int64{2}
			assert.Equal(t, pinned, expected)
		}
	}
}
