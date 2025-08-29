// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCanUserForkBetweenOwners(t *testing.T) {
	defer test.MockVariableValue(&setting.Repository.AllowForkIntoSameOwner)

	setting.Repository.AllowForkIntoSameOwner = true
	assert.True(t, CanUserForkBetweenOwners(1, 1))
	assert.True(t, CanUserForkBetweenOwners(1, 2))

	setting.Repository.AllowForkIntoSameOwner = false
	assert.False(t, CanUserForkBetweenOwners(1, 1))
	assert.True(t, CanUserForkBetweenOwners(1, 2))
}
