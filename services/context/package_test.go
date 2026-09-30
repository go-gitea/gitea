// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	"gitea.dev/models/perm"
	"gitea.dev/models/user"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestDeterminePackageAccessModeForLimitedOwner(t *testing.T) {
	owner := &user.User{ID: 1, Visibility: structs.VisibleTypeLimited}

	accessMode, err := determineAccessMode(&Base{}, owner, &user.User{ID: 2, IsActive: true})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeRead, accessMode)

	accessMode, err = determineAccessMode(&Base{}, owner, &user.User{ID: 3, IsActive: true, IsRestricted: true})
	assert.NoError(t, err)
	assert.Equal(t, perm.AccessModeNone, accessMode)
}
