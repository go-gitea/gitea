// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/optional"

	"github.com/stretchr/testify/assert"
)

func TestGetSystemOrDefaultWebhooks(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	hooks, err := GetSystemOrDefaultWebhooks(db.DefaultContext, optional.None[bool]())
	assert.NoError(t, err)
	if assert.Len(t, hooks, 2) {
		assert.Equal(t, int64(5), hooks[0].ID)
		assert.Equal(t, int64(6), hooks[1].ID)
	}

	hooks, err = GetSystemOrDefaultWebhooks(db.DefaultContext, optional.Some(true))
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(5), hooks[0].ID)
	}

	hooks, err = GetSystemOrDefaultWebhooks(db.DefaultContext, optional.Some(false))
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(6), hooks[0].ID)
	}
}
