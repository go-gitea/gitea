// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadRepositoryCreationLimits(t *testing.T) {
	defer test.MockVariableValue(&Repository.MaxCreationLimit)()
	defer test.MockVariableValue(&Repository.UserMaxCreationLimit)()
	defer test.MockVariableValue(&Repository.OrgMaxCreationLimit)()

	t.Run("ShortcutPropagatesToBoth", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[repository]
MAX_CREATION_LIMIT = 5
`)
		assert.NoError(t, err)
		loadRepositoryFrom(cfg)
		assert.Equal(t, 5, Repository.MaxCreationLimit)
		assert.Equal(t, 5, Repository.UserMaxCreationLimit)
		assert.Equal(t, 5, Repository.OrgMaxCreationLimit)
	})

	t.Run("PerTypeKeysOverrideShortcut", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[repository]
MAX_CREATION_LIMIT = 5
USER_MAX_CREATION_LIMIT = 0
ORG_MAX_CREATION_LIMIT = -1
`)
		assert.NoError(t, err)
		loadRepositoryFrom(cfg)
		assert.Equal(t, 0, Repository.UserMaxCreationLimit)
		assert.Equal(t, -1, Repository.OrgMaxCreationLimit)
	})

	t.Run("PartialOverrideOtherInheritsShortcut", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[repository]
MAX_CREATION_LIMIT = 7
ORG_MAX_CREATION_LIMIT = -1
`)
		assert.NoError(t, err)
		loadRepositoryFrom(cfg)
		assert.Equal(t, 7, Repository.UserMaxCreationLimit)
		assert.Equal(t, -1, Repository.OrgMaxCreationLimit)
	})

	t.Run("NoKeyDefaultsToNoLimit", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[repository]
`)
		assert.NoError(t, err)
		loadRepositoryFrom(cfg)
		assert.Equal(t, -1, Repository.MaxCreationLimit)
		assert.Equal(t, -1, Repository.UserMaxCreationLimit)
		assert.Equal(t, -1, Repository.OrgMaxCreationLimit)
	})
}
