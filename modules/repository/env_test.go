// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendGitConfigEnv(t *testing.T) {
	t.Run("adds first config", func(t *testing.T) {
		env := appendGitConfigEnv([]string{"A=B"}, "receive.hideRefs", "!refs/pull/")

		assert.Contains(t, env, "A=B")
		assert.Contains(t, env, "GIT_CONFIG_COUNT=1")
		assert.Contains(t, env, "GIT_CONFIG_KEY_0=receive.hideRefs")
		assert.Contains(t, env, "GIT_CONFIG_VALUE_0=!refs/pull/")
	})

	t.Run("appends after existing config", func(t *testing.T) {
		env := appendGitConfigEnv([]string{
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=core.editor",
			"GIT_CONFIG_VALUE_0=vim",
		}, "receive.hideRefs", "!refs/pull/")

		assert.Contains(t, env, "GIT_CONFIG_COUNT=2")
		assert.Contains(t, env, "GIT_CONFIG_KEY_0=core.editor")
		assert.Contains(t, env, "GIT_CONFIG_VALUE_0=vim")
		assert.Contains(t, env, "GIT_CONFIG_KEY_1=receive.hideRefs")
		assert.Contains(t, env, "GIT_CONFIG_VALUE_1=!refs/pull/")
	})
}
