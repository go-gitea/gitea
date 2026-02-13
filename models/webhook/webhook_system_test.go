// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/optional"

	"github.com/stretchr/testify/assert"
)

func TestListSystemWebhookOptions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	opts := ListSystemWebhookOptions{IsSystem: optional.None[bool]()}
	hooks, _, err := GetGlobalWebhooks(t.Context(), &opts)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 2) {
		assert.Equal(t, int64(5), hooks[0].ID)
		assert.Equal(t, int64(6), hooks[1].ID)
	}
	opts.IsSystem = optional.Some(true)
	hooks, _, err = GetGlobalWebhooks(t.Context(), &opts)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(5), hooks[0].ID)
	}

	opts.IsSystem = optional.Some(false)
	hooks, _, err = GetGlobalWebhooks(t.Context(), &opts)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(6), hooks[0].ID)
	}
}
