// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/optional"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListSystemWebhookOptions(t *testing.T) {
	hookSystem := unittest.AssertExistsAndLoadBean(t, &Webhook{URL: "https://www.example.com/system"})
	hookDefault := unittest.AssertExistsAndLoadBean(t, &Webhook{URL: "https://www.example.com/default"})
	opts := ListSystemWebhookOptions{IsSystem: optional.None[bool]()}
	hooks, _, err := GetGlobalWebhooks(t.Context(), &opts)
	require.NoError(t, err)
	require.Len(t, hooks, 2)
	assert.Equal(t, hookSystem.ID, hooks[0].ID)
	assert.Equal(t, hookDefault.ID, hooks[1].ID)

	opts.IsSystem = optional.Some(true)
	hooks, _, err = GetGlobalWebhooks(t.Context(), &opts)
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	assert.Equal(t, hookSystem.ID, hooks[0].ID)

	opts.IsSystem = optional.Some(false)
	hooks, _, err = GetGlobalWebhooks(t.Context(), &opts)
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	assert.Equal(t, hookDefault.ID, hooks[0].ID)
}
